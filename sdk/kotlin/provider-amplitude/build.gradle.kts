import com.vanniktech.maven.publish.SonatypeHost

plugins {
    alias(libs.plugins.kotlin.jvm)
    alias(libs.plugins.kotlin.serialization)
    alias(libs.plugins.vanniktech.publish)
    alias(libs.plugins.ktfmt)
}

group = "io.event-spec"
version = project.findProperty("releaseVersion") as? String ?: "0.1.0"

repositories {
    mavenCentral()
}

dependencies {
    implementation(project(":api"))
    implementation(libs.kotlinx.coroutines.core)
    implementation(libs.kotlinx.serialization.json)
    testImplementation(kotlin("test"))
    testImplementation(libs.kotlinx.coroutines.test)
    testImplementation(project(":api"))
}

tasks.test {
    useJUnitPlatform()
}

kotlin {
    jvmToolchain(17)
}

mavenPublishing {
    publishToMavenCentral(SonatypeHost.CENTRAL_PORTAL)
    signAllPublications()

    coordinates("io.event-spec", "kotlin-provider-amplitude", version.toString())

    pom {
        name = "event-spec Kotlin Amplitude Provider"
        description =
            "Amplitude analytics provider for event-spec — batched track via EventQueue, synchronous identify/group/alias"
        url = "https://event-spec.io"
        licenses {
            license {
                name = "Apache-2.0"
                url = "https://www.apache.org/licenses/LICENSE-2.0"
            }
        }
        developers {
            developer {
                id = "dejanradmanovic"
                name = "Dejan Radmanovic"
                email = "dejan.radmanovic@vicert.com"
            }
        }
        scm {
            connection = "scm:git:git://github.com/dejanradmanovic/event-spec.git"
            developerConnection = "scm:git:ssh://github.com/dejanradmanovic/event-spec.git"
            url = "https://github.com/dejanradmanovic/event-spec"
        }
    }
}
