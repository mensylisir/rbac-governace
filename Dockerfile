FROM node:26-alpine AS frontend
WORKDIR /src/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.26-alpine AS backend
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY --from=frontend /src/web/dist ./web/dist
RUN go build -buildvcs=false -o /out/rbac-manager ./cmd/server

FROM alpine:3.22
RUN adduser -D -H -u 10001 app
WORKDIR /app
COPY --from=backend /out/rbac-manager /app/rbac-manager
COPY --from=frontend /src/web/dist /app/web/dist
USER app
EXPOSE 8080
ENV ADDR=:8080
CMD ["/app/rbac-manager"]

