# ─── Stage 1: build com Maven ────────────────────────────────────────────────
FROM maven:3.9-eclipse-temurin-17 AS builder

WORKDIR /build

# cache das dependências
COPY pom.xml .
RUN mvn dependency:go-offline -q

# build
COPY src ./src
RUN mvn package -q -DskipTests

# ─── Stage 2: runtime ─────────────────────────────────────────────────────────
FROM eclipse-temurin:17-jre

WORKDIR /app

COPY --from=builder /build/target/*.jar app.jar
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=5s --start-period=90s --retries=12 \
    CMD curl -sf http://localhost:8080/probe || exit 1

ENTRYPOINT ["/entrypoint.sh"]
