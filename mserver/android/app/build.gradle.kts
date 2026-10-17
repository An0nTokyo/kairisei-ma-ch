plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
}

android {
    namespace = "local.kairisei.mserver"
    compileSdk = 34

    defaultConfig {
        applicationId = "local.kairisei.mserver"
        minSdk = 26
        targetSdk = 34
        versionCode = 1
        versionName = "1.0"
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    kotlinOptions {
        jvmTarget = "17"
    }

    sourceSets.getByName("main").assets.srcDir(layout.buildDirectory.dir("generated/seedAssets"))
}

val stageSeed = tasks.register<Copy>("stageSeed") {
    from(rootProject.file("../../server/config/cn602-save-template.json"))
    into(layout.buildDirectory.dir("generated/seedAssets"))
    rename { "unused-seed.json" }
}

tasks.named("preBuild").configure { dependsOn(stageSeed) }

dependencies {
    testImplementation("junit:junit:4.13.2")
}
