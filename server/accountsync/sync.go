package accountsync

import (
	"bytes"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"kairisei.local/server/internal/accountstore"
)

const (
	ServerHolderID = "s"
	tokenHeader    = "X-Account-Sync-Token"
	maxBody        = 128 << 20
)

type HolderView struct {
	HolderID         string
	CheckoutRevision int
	MasterDigest     string
	Frozen           bool
}

type DB interface {
	Holder(userID int) (HolderView, error)
	SaveHolder(userID int, holderID string, checkoutRevision int, masterDigest string, frozen bool) error
	Revision(userID int) (int, error)
	Export(userID int) ([]byte, error)
	Import(raw []byte) error
	LoginUUID(userID int) (string, error)
	DeliverHeldMail(userID int) error
	MasterFiles() (cards, growth []byte, err error)
}

type Phone interface {
	EnsureLogin(userID int, loginUUID, session string) error
	Import(raw []byte) error
	Export(userID int) ([]byte, error)
	Holder(userID int) (HolderView, error)
	SaveHolder(userID int, holderID string, checkoutRevision int, masterDigest string, frozen bool) error
	ReadMasters() (cards, growth []byte, ok bool, err error)
	WriteMasters(cards, growth []byte) error
}

type ChoiceRequiredError struct {
	ServerRevision int
	PhoneRevision  int
}

func (e *ChoiceRequiredError) Error() string { return "choice required" }

type Service struct {
	DB DB
}

type borrowResponse struct {
	Transfer     []byte `json:"transfer"`
	LoginUUID    string `json:"login_uuid"`
	Cards        []byte `json:"cards"`
	Growth       []byte `json:"growth"`
	Revision     int    `json:"revision"`
	MasterDigest string `json:"master_digest"`
}

func (service Service) PrepareBorrow(userID int, deviceID string) (borrowResponse, error) {
	if userID < accountstore.PrimaryUserID || deviceID == "" {
		return borrowResponse{}, errors.New("account borrow is invalid")
	}
	holder, err := service.DB.Holder(userID)
	if err != nil && !strings.Contains(err.Error(), "missing") {
		return borrowResponse{}, err
	}
	if err == nil && holder.HolderID != ServerHolderID {
		return borrowResponse{}, errors.New("account is not on the server")
	}
	cards, growth, err := service.DB.MasterFiles()
	if err != nil {
		return borrowResponse{}, err
	}
	digest := accountstore.MasterDigest(cards, growth)
	revision, err := service.DB.Revision(userID)
	if err != nil {
		return borrowResponse{}, err
	}
	if err := service.DB.SaveHolder(userID, ServerHolderID, revision, digest, true); err != nil {
		return borrowResponse{}, err
	}
	raw, err := service.DB.Export(userID)
	if err != nil {
		_ = service.DB.SaveHolder(userID, ServerHolderID, 0, "", false)
		return borrowResponse{}, err
	}
	annotated, revision, err := setMasterDigest(raw, digest)
	if err != nil {
		_ = service.DB.SaveHolder(userID, ServerHolderID, 0, "", false)
		return borrowResponse{}, err
	}
	loginUUID, err := service.DB.LoginUUID(userID)
	if err != nil {
		_ = service.DB.SaveHolder(userID, ServerHolderID, 0, "", false)
		return borrowResponse{}, err
	}
	return borrowResponse{
		Transfer:     annotated,
		LoginUUID:    loginUUID,
		Cards:        cards,
		Growth:       growth,
		Revision:     revision,
		MasterDigest: digest,
	}, nil
}

func (service Service) AbortBorrow(userID int) error {
	holder, err := service.DB.Holder(userID)
	if err != nil {
		if strings.Contains(err.Error(), "missing") {
			return nil
		}
		return err
	}
	if holder.HolderID != ServerHolderID {
		return errors.New("account is not on the server")
	}
	return service.DB.SaveHolder(userID, ServerHolderID, 0, "", false)
}

func (service Service) CommitBorrow(userID int, deviceID string) error {
	if deviceID == "" || deviceID == ServerHolderID {
		return errors.New("account borrow device is invalid")
	}
	holder, err := service.DB.Holder(userID)
	if err != nil {
		return err
	}
	if holder.HolderID != ServerHolderID || !holder.Frozen {
		return errors.New("account borrow is not waiting")
	}
	return service.DB.SaveHolder(userID, deviceID, holder.CheckoutRevision, holder.MasterDigest, false)
}

func (service Service) AcceptReturn(userID int, deviceID, choice string, transfer []byte) error {
	holder, err := service.DB.Holder(userID)
	if err != nil {
		return err
	}
	if holder.HolderID != deviceID || holder.Frozen {
		return errors.New("account is not on the server")
	}
	document, err := readTransfer(transfer)
	if err != nil {
		return err
	}
	if document.MasterDigest != holder.MasterDigest {
		return errors.New("CN account master digest mismatch")
	}
	revision, err := service.DB.Revision(userID)
	if err != nil {
		return err
	}
	if revision != holder.CheckoutRevision && choice == "" {
		return &ChoiceRequiredError{ServerRevision: revision, PhoneRevision: document.Revision}
	}
	if choice == "server" {
		if err := service.DB.DeliverHeldMail(userID); err != nil {
			return err
		}
		return service.DB.SaveHolder(userID, ServerHolderID, 0, "", false)
	}
	if choice != "" && choice != "phone" {
		return errors.New("account return choice is invalid")
	}
	if err := service.DB.Import(transfer); err != nil {
		return err
	}
	if err := service.DB.DeliverHeldMail(userID); err != nil {
		return err
	}
	return service.DB.SaveHolder(userID, ServerHolderID, 0, "", false)
}

func Handler(token string, db DB) http.Handler {
	service := Service{DB: db}
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || token == "" || !tokenOK(request.Header.Get(tokenHeader), token) {
			http.NotFound(writer, request)
			return
		}
		body, err := io.ReadAll(io.LimitReader(request.Body, maxBody+1))
		if err != nil || len(body) > maxBody {
			writeError(writer, http.StatusBadRequest, "account sync body is invalid")
			return
		}
		switch request.URL.Path {
		case "/local/sync/borrow":
			var payload struct {
				UserID   int    `json:"user_id"`
				DeviceID string `json:"device_id"`
			}
			if err := json.Unmarshal(body, &payload); err != nil {
				writeError(writer, http.StatusBadRequest, "account sync body is invalid")
				return
			}
			result, err := service.PrepareBorrow(payload.UserID, payload.DeviceID)
			if err != nil {
				writeError(writer, http.StatusConflict, err.Error())
				return
			}
			writeJSON(writer, http.StatusOK, result)
		case "/local/sync/borrow/abort":
			var payload struct {
				UserID int `json:"user_id"`
			}
			if err := json.Unmarshal(body, &payload); err != nil {
				writeError(writer, http.StatusBadRequest, "account sync body is invalid")
				return
			}
			if err := service.AbortBorrow(payload.UserID); err != nil {
				writeError(writer, http.StatusConflict, err.Error())
				return
			}
			writeJSON(writer, http.StatusOK, map[string]bool{"ok": true})
		case "/local/sync/borrow/commit":
			var payload struct {
				UserID   int    `json:"user_id"`
				DeviceID string `json:"device_id"`
			}
			if err := json.Unmarshal(body, &payload); err != nil {
				writeError(writer, http.StatusBadRequest, "account sync body is invalid")
				return
			}
			if err := service.CommitBorrow(payload.UserID, payload.DeviceID); err != nil {
				writeError(writer, http.StatusConflict, err.Error())
				return
			}
			writeJSON(writer, http.StatusOK, map[string]bool{"ok": true})
		case "/local/sync/return":
			var payload struct {
				UserID   int    `json:"user_id"`
				DeviceID string `json:"device_id"`
				Choice   string `json:"choice"`
				Transfer []byte `json:"transfer"`
			}
			if err := json.Unmarshal(body, &payload); err != nil {
				writeError(writer, http.StatusBadRequest, "account sync body is invalid")
				return
			}
			err := service.AcceptReturn(payload.UserID, payload.DeviceID, payload.Choice, payload.Transfer)
			var choice *ChoiceRequiredError
			if errors.As(err, &choice) {
				writeJSON(writer, http.StatusConflict, map[string]any{
					"error":           "choice required",
					"server_revision": choice.ServerRevision,
					"phone_revision":  choice.PhoneRevision,
				})
				return
			}
			if err != nil {
				writeError(writer, http.StatusConflict, err.Error())
				return
			}
			writeJSON(writer, http.StatusOK, map[string]bool{"ok": true})
		default:
			http.NotFound(writer, request)
		}
	})
}

func Borrow(baseURL, token, deviceID string, userID int, phone Phone) error {
	var result borrowResponse
	if err := post(baseURL, token, "/local/sync/borrow", map[string]any{
		"user_id":   userID,
		"device_id": deviceID,
	}, &result, nil); err != nil {
		return err
	}
	if accountstore.MasterDigest(result.Cards, result.Growth) != result.MasterDigest {
		_ = abort(baseURL, token, userID)
		return errors.New("CN account master digest mismatch")
	}
	cards, growth, ok, err := phone.ReadMasters()
	if err != nil {
		_ = abort(baseURL, token, userID)
		return err
	}
	if ok && accountstore.MasterDigest(cards, growth) != result.MasterDigest {
		_ = abort(baseURL, token, userID)
		return errors.New("CN account master digest mismatch")
	}
	if !ok {
		if err := phone.WriteMasters(result.Cards, result.Growth); err != nil {
			_ = abort(baseURL, token, userID)
			return err
		}
	}
	if err := phone.EnsureLogin(userID, result.LoginUUID, "borrowed-"+result.LoginUUID); err != nil {
		_ = abort(baseURL, token, userID)
		return err
	}
	if err := phone.Import(result.Transfer); err != nil {
		_ = abort(baseURL, token, userID)
		return err
	}
	if err := post(baseURL, token, "/local/sync/borrow/commit", map[string]any{
		"user_id":   userID,
		"device_id": deviceID,
	}, nil, nil); err != nil {
		return err
	}
	return phone.SaveHolder(userID, deviceID, result.Revision, result.MasterDigest, false)
}

func Return(baseURL, token, deviceID string, userID int, choice string, phone Phone) error {
	holder, err := phone.Holder(userID)
	if err != nil {
		return err
	}
	if holder.HolderID != deviceID || holder.Frozen {
		return errors.New("account is not on the phone")
	}
	cards, growth, ok, err := phone.ReadMasters()
	if err != nil {
		return err
	}
	if !ok || accountstore.MasterDigest(cards, growth) != holder.MasterDigest {
		return errors.New("CN account master digest mismatch")
	}
	if err := phone.SaveHolder(userID, deviceID, holder.CheckoutRevision, holder.MasterDigest, true); err != nil {
		return err
	}
	restore := func() {
		_ = phone.SaveHolder(userID, deviceID, holder.CheckoutRevision, holder.MasterDigest, false)
	}
	raw, err := phone.Export(userID)
	if err != nil {
		restore()
		return err
	}
	annotated, _, err := setMasterDigest(raw, holder.MasterDigest)
	if err != nil {
		restore()
		return err
	}
	var choiceError *ChoiceRequiredError
	err = post(baseURL, token, "/local/sync/return", map[string]any{
		"user_id":   userID,
		"device_id": deviceID,
		"choice":    choice,
		"transfer":  annotated,
	}, nil, &choiceError)
	if choiceError != nil {
		restore()
		return choiceError
	}
	if err != nil {
		restore()
		return err
	}
	return phone.SaveHolder(userID, ServerHolderID, 0, "", false)
}

func abort(baseURL, token string, userID int) error {
	return post(baseURL, token, "/local/sync/borrow/abort", map[string]any{"user_id": userID}, nil, nil)
}

func post(baseURL, token, path string, body any, dest any, choice **ChoiceRequiredError) error {
	encoded, err := json.Marshal(body)
	if err != nil {
		return err
	}
	request, err := http.NewRequest(http.MethodPost, strings.TrimRight(baseURL, "/")+path, bytes.NewReader(encoded))
	if err != nil {
		return err
	}
	request.Header.Set(tokenHeader, token)
	request.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 2 * time.Minute}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(response.Body, maxBody+1))
	if err != nil {
		return err
	}
	if response.StatusCode == http.StatusConflict {
		var decoded struct {
			Error          string `json:"error"`
			ServerRevision int    `json:"server_revision"`
			PhoneRevision  int    `json:"phone_revision"`
		}
		if json.Unmarshal(payload, &decoded) == nil && decoded.Error == "choice required" && choice != nil {
			*choice = &ChoiceRequiredError{ServerRevision: decoded.ServerRevision, PhoneRevision: decoded.PhoneRevision}
			return nil
		}
	}
	if response.StatusCode != http.StatusOK {
		var decoded struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(payload, &decoded) == nil && decoded.Error != "" {
			return errors.New(decoded.Error)
		}
		return errors.New("account sync failed")
	}
	if dest != nil && len(payload) > 0 {
		return json.Unmarshal(payload, dest)
	}
	return nil
}

type transferDocument struct {
	UserID         int             `json:"UserID"`
	Revision       int             `json:"Revision"`
	Metadata       []byte          `json:"Metadata"`
	MetadataDigest string          `json:"MetadataDigest"`
	MasterDigest   string          `json:"MasterDigest"`
	Account        json.RawMessage `json:"Account"`
}

func setMasterDigest(raw []byte, digest string) ([]byte, int, error) {
	var document transferDocument
	if err := json.Unmarshal(raw, &document); err != nil {
		return nil, 0, err
	}
	document.MasterDigest = digest
	encoded, err := json.Marshal(document)
	if err != nil {
		return nil, 0, err
	}
	return encoded, document.Revision, nil
}

func readTransfer(raw []byte) (transferDocument, error) {
	var document transferDocument
	err := json.Unmarshal(raw, &document)
	return document, err
}

func tokenOK(got, want string) bool {
	if want == "" || len(got) != len(want) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

func writeJSON(writer http.ResponseWriter, status int, body any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(body)
}

func writeError(writer http.ResponseWriter, status int, message string) {
	writeJSON(writer, status, map[string]string{"error": message})
}
