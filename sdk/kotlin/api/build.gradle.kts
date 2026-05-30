import com.vanniktech.maven.publish.SonatypeHost

plugins {
    alias(libs.plugins.kotlin.jvm)
    alias(libs.plugins.vanniktech.publish)
    alias(libs.plugins.ktfmt)
}

group = "io.event-spec"
version = project.findProperty("releaseVersion") as? String ?: "0.1.0"

repositories {
    mavenCentral()
}

dependencies {
    implementation(libs.kotlinx.coroutines.core)
    testImplementation(kotlin("test"))
    testImplementation(libs.kotlinx.coroutines.test)
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

    coordinates("io.event-spec", "kotlin-api", version.toString())

    pom {
        name = "event-spec Kotlin API"
        description =
            "Kotlin core runtime for event-spec — Provider interface, Hook chain, Client dispatch, sampling and validation hooks"
        url = "https://github.com/dejanradmanovic/event-spec"
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
