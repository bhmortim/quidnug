plugins {
    `java-library`
    `maven-publish`
}

group = "com.quidnug"
version = "2.0.0"
description = "Quidnug Java client SDK"

java {
    toolchain {
        languageVersion = JavaLanguageVersion.of(17)
    }
    withSourcesJar()
    withJavadocJar()
}

repositories {
    mavenCentral()
}

dependencies {
    implementation("com.fasterxml.jackson.core:jackson-databind:2.18.1")
    implementation("org.bouncycastle:bcprov-jdk18on:1.78.1")
    // JDK 11+ has java.net.http.HttpClient built in.

    testImplementation("org.junit.jupiter:junit-jupiter:5.11.3")
    testImplementation("org.mockito:mockito-core:5.14.2")
}

tasks.test {
    useJUnitPlatform()
}

publishing {
    publications {
        create<MavenPublication>("maven") {
            from(components["java"])
            pom {
                name.set("Quidnug Java Client")
                description.set(project.description)
                url.set("https://github.com/bhmortim/quidnug")
                licenses {
                    license {
                        name.set("The Apache License, Version 2.0")
                        url.set("http://www.apache.org/licenses/LICENSE-2.0.txt")
                    }
                }
            }
        }
    }
}
