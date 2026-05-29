plugins {
    kotlin("jvm") version "2.1.20"
    `maven-publish`
    id("com.ncorti.ktfmt.gradle") version "0.20.1"
}

group = "io.event-spec"
version = "0.1.0"

repositories {
    mavenCentral()
}

dependencies {
    implementation(project(":api"))
    implementation("org.jetbrains.kotlinx:kotlinx-coroutines-core:1.9.0")
    testImplementation(kotlin("test"))
    testImplementation("org.jetbrains.kotlinx:kotlinx-coroutines-test:1.9.0")
    testImplementation(project(":api"))
}

tasks.test {
    useJUnitPlatform()
}

kotlin {
    jvmToolchain(17)
}

publishing {
    publications {
        create<MavenPublication>("maven") {
            from(components["java"])
            groupId = "io.event-spec"
            artifactId = "kotlin-provider-amplitude"
        }
    }
}
