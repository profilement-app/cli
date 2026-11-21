package scan

import (
	"regexp"
	"strings"
)

// images are the official images people name a product by ("php:8.3-fpm" is PHP 8.3).
var images = map[string]string{
	"php": "PHP", "node": "Node.js", "python": "Python", "golang": "Go", "ruby": "Ruby", "openjdk": "Java",
	"eclipse-temurin": "Java", "amazoncorretto": "Java", "maven": "Maven", "gradle": "Gradle", "rust": "Rust",
	"mcr.microsoft.com/dotnet/aspnet": ".NET", "mcr.microsoft.com/dotnet/sdk": ".NET", "nginx": "nginx",
	"httpd": "Apache HTTP Server", "caddy": "Caddy", "traefik": "Traefik", "haproxy": "HAProxy", "varnish": "Varnish",
	"mysql": "MySQL", "mariadb": "MariaDB", "postgres": "PostgreSQL", "postgis/postgis": "PostGIS", "mongo": "MongoDB",
	"redis": "Redis", "valkey/valkey": "Valkey", "memcached": "Memcached", "elasticsearch": "Elasticsearch",
	"docker.elastic.co/elasticsearch/elasticsearch": "Elasticsearch", "opensearchproject/opensearch": "OpenSearch",
	"solr": "Apache Solr", "rabbitmq": "RabbitMQ", "bitnami/kafka": "Apache Kafka", "confluentinc/cp-kafka": "Apache Kafka",
	"apache/kafka": "Apache Kafka", "minio/minio": "MinIO", "mailhog/mailhog": "MailHog", "axllent/mailpit": "Mailpit",
	"schickling/mailcatcher": "MailCatcher", "phpmyadmin": "phpMyAdmin", "adminer": "Adminer", "keycloak/keycloak": "Keycloak",
	"quay.io/keycloak/keycloak": "Keycloak", "grafana/grafana": "Grafana", "prom/prometheus": "Prometheus",
	"jenkins/jenkins": "Jenkins", "sonarqube": "SonarQube", "selenium/standalone-chrome": "Selenium", "wordpress": "WordPress",
	"drupal": "Drupal", "nextcloud": "Nextcloud", "alpine": "Alpine Linux", "ubuntu": "Ubuntu", "debian": "Debian",
	"composer": "Composer", "cosyses/ubuntu": "Ubuntu", "mcr.microsoft.com/mssql/server": "Microsoft SQL Server",
	"gvenzl/oracle-xe": "Oracle Database", "clickhouse/clickhouse-server": "ClickHouse", "influxdb": "InfluxDB",
	"neo4j": "Neo4j", "cassandra": "Apache Cassandra", "couchdb": "CouchDB", "localstack/localstack": "LocalStack",
}

var (
	fromLine  = regexp.MustCompile(`(?mi)^\s*FROM\s+(?:--platform=\S+\s+)?(\S+)`)
	imageLine = regexp.MustCompile(`(?m)^\s*image:\s*['"]?([^'"\s#]+)`)
)

// imageTerm is the product an image reference names, with the version its tag starts with.
func imageTerm(reference, relative string) (Term, bool) {
	reference = strings.ToLower(reference)

	if strings.Contains(reference, "$") {
		return Term{}, false
	}

	name, tag, _ := strings.Cut(strings.Split(reference, "@")[0], ":")
	// Docker Hub's official images may be written in full ("docker.io/library/php").
	name = strings.TrimPrefix(strings.TrimPrefix(name, "docker.io/"), "library/")
	product, ok := images[name]

	if !ok {
		return Term{}, false
	}

	return Term{Name: product, Package: name, Version: versionOf(tag), File: relative}, true
}

// dockerfile reads a Dockerfile's images.
func dockerfile(path, relative string) []Term {
	terms := []Term{{Name: "Docker", File: relative}}

	for _, match := range fromLine.FindAllStringSubmatch(readFile(path), -1) {
		if term, ok := imageTerm(match[1], relative); ok {
			terms = append(terms, term)
		}
	}

	return terms
}

// isComposeFile is a Docker Compose file's name.
func isComposeFile(lower string) bool {
	for _, prefix := range []string{"docker-compose", "compose"} {
		if strings.HasPrefix(lower, prefix) && (strings.HasSuffix(lower, ".yml") || strings.HasSuffix(lower, ".yaml")) {
			return true
		}
	}

	return false
}

// composeFile reads a Compose file's images.
func composeFile(path, relative string) []Term {
	terms := []Term{{Name: "Docker", File: relative}, {Name: "Docker Compose", File: relative}}

	for _, match := range imageLine.FindAllStringSubmatch(readFile(path), -1) {
		if term, ok := imageTerm(match[1], relative); ok {
			terms = append(terms, term)
		}
	}

	return terms
}
