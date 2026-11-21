package scan

import (
	"regexp"
	"strings"
)

// knownNames are the names people write for well-known packages, per ecosystem; a key ending in "*" matches every
// package starting with it ("symfony/*"). Anything else gets a name from the package itself (fallbackName()), which
// Profilement matches against its catalog as it is, and by the package name too.
var knownNames = map[string]map[string]string{
	"composer": {
		"laravel/framework": "Laravel", "symfony/symfony": "Symfony", "symfony/framework-bundle": "Symfony",
		"livewire/livewire": "Livewire", "livewire/flux": "Flux", "phpunit/phpunit": "PHPUnit", "phpstan/phpstan": "PHPStan",
		"larastan/larastan": "Larastan", "friendsofphp/php-cs-fixer": "PHP-CS-Fixer", "squizlabs/php_codesniffer": "PHP_CodeSniffer",
		"doctrine/orm": "Doctrine ORM", "doctrine/dbal": "Doctrine DBAL", "twig/twig": "Twig", "guzzlehttp/guzzle": "Guzzle",
		"monolog/monolog": "Monolog", "magento/product-community-edition": "Magento", "magento/product-enterprise-edition": "Adobe Commerce",
		"shopware/core": "Shopware", "shopware/platform": "Shopware", "pimcore/pimcore": "Pimcore", "drupal/core": "Drupal",
		"drupal/core-recommended": "Drupal", "typo3/cms-core": "TYPO3", "laravel/sanctum": "Laravel Sanctum",
		"laravel/horizon": "Laravel Horizon", "laravel/octane": "Laravel Octane", "laravel/pint": "Laravel Pint",
		"laravel/sail": "Laravel Sail", "laravel/passport": "Laravel Passport", "laravel/scout": "Laravel Scout",
		"laravel/socialite": "Laravel Socialite", "laravel/telescope": "Laravel Telescope", "laravel/tinker": "Laravel Tinker",
		"pestphp/pest": "Pest", "mockery/mockery": "Mockery", "rector/rector": "Rector", "vimeo/psalm": "Psalm",
		"api-platform/core": "API Platform", "slim/slim": "Slim", "cakephp/cakephp": "CakePHP", "yiisoft/yii2": "Yii",
		"codeigniter4/framework": "CodeIgniter", "predis/predis": "Predis", "elasticsearch/elasticsearch": "Elasticsearch",
		"opensearch-project/opensearch-php": "OpenSearch", "aws/aws-sdk-php": "AWS SDK for PHP", "stripe/stripe-php": "Stripe",
		"sylius/sylius": "Sylius", "oro/platform": "OroPlatform", "contao/core-bundle": "Contao", "neos/neos": "Neos",
		"craftcms/cms": "Craft CMS", "sulu/sulu": "Sulu", "akeneo/pim-community-dev": "Akeneo PIM", "behat/behat": "Behat",
		"codeception/codeception": "Codeception", "deployer/deployer": "Deployer", "spatie/laravel-permission": "Laravel Permission",
		"filament/filament": "Filament", "inertiajs/inertia-laravel": "Inertia.js", "nesbot/carbon": "Carbon",
		"league/flysystem": "Flysystem", "phpoffice/phpspreadsheet": "PhpSpreadsheet", "dompdf/dompdf": "Dompdf",
		"mpdf/mpdf": "mPDF", "tecnickcom/tcpdf": "TCPDF", "swiftmailer/swiftmailer": "Swift Mailer", "phpmailer/phpmailer": "PHPMailer",
		"ramsey/uuid": "ramsey/uuid", "firebase/php-jwt": "JWT", "lcobucci/jwt": "JWT", "league/oauth2-server": "OAuth 2.0",
		"php-amqplib/php-amqplib": "RabbitMQ", "anthropic-ai/sdk": "Anthropic API", "openai-php/client": "OpenAI API",
		"symfony/*": "Symfony", "doctrine/*": "Doctrine",
	},
	"npm": {
		"react": "React", "react-dom": "React", "vue": "Vue.js", "@angular/core": "Angular", "svelte": "Svelte", "next": "Next.js",
		"nuxt": "Nuxt", "typescript": "TypeScript", "vite": "Vite", "webpack": "webpack", "tailwindcss": "Tailwind CSS",
		"bootstrap": "Bootstrap", "jquery": "jQuery", "axios": "Axios", "express": "Express", "@nestjs/core": "NestJS",
		"jest": "Jest", "vitest": "Vitest", "@playwright/test": "Playwright", "playwright": "Playwright", "cypress": "Cypress",
		"eslint": "ESLint", "prettier": "Prettier", "sass": "Sass", "node-sass": "Sass", "less": "Less", "redux": "Redux",
		"@reduxjs/toolkit": "Redux", "rxjs": "RxJS", "lodash": "Lodash", "alpinejs": "Alpine.js", "daisyui": "daisyUI",
		"three": "three.js", "d3": "D3.js", "chart.js": "Chart.js", "socket.io": "Socket.IO", "graphql": "GraphQL",
		"@apollo/client": "Apollo Client", "@apollo/server": "Apollo Server", "mongoose": "Mongoose", "prisma": "Prisma",
		"@prisma/client": "Prisma", "sequelize": "Sequelize", "typeorm": "TypeORM", "storybook": "Storybook",
		"electron": "Electron", "react-native": "React Native", "@capacitor/core": "Capacitor", "@ionic/core": "Ionic",
		"gsap": "GSAP", "@vue/cli-service": "Vue CLI", "pinia": "Pinia", "vuex": "Vuex", "@tanstack/react-query": "TanStack Query",
		"zustand": "Zustand", "@mui/material": "Material UI", "antd": "Ant Design", "@emotion/react": "Emotion",
		"styled-components": "styled-components", "gatsby": "Gatsby", "astro": "Astro", "remix": "Remix", "@remix-run/react": "Remix",
		"solid-js": "SolidJS", "preact": "Preact", "lit": "Lit", "stimulus": "Stimulus", "@hotwired/turbo": "Turbo",
		"htmx.org": "htmx", "knockout": "Knockout", "backbone": "Backbone.js", "ember-source": "Ember.js", "mocha": "Mocha",
		"chai": "Chai", "karma": "Karma", "jasmine-core": "Jasmine", "@testing-library/react": "Testing Library",
		"puppeteer": "Puppeteer", "webdriverio": "WebdriverIO", "esbuild": "esbuild", "rollup": "Rollup", "parcel": "Parcel",
		"gulp": "gulp", "grunt": "Grunt", "babel-loader": "Babel", "@babel/core": "Babel", "stylelint": "Stylelint",
		"husky": "Husky", "lerna": "Lerna", "nx": "Nx", "turbo": "Turborepo", "fastify": "Fastify", "koa": "Koa",
		"hapi": "hapi", "@hapi/hapi": "hapi", "pg": "PostgreSQL", "mysql2": "MySQL", "redis": "Redis", "ioredis": "Redis",
		"kafkajs": "Apache Kafka", "amqplib": "RabbitMQ", "openai": "OpenAI API", "@anthropic-ai/sdk": "Anthropic API",
		"@sentry/browser": "Sentry", "@sentry/node": "Sentry",
	},
	"pypi": {
		"django": "Django", "flask": "Flask", "fastapi": "FastAPI", "pandas": "pandas", "numpy": "NumPy", "scipy": "SciPy",
		"scikit-learn": "scikit-learn", "tensorflow": "TensorFlow", "torch": "PyTorch", "pytest": "pytest", "celery": "Celery",
		"sqlalchemy": "SQLAlchemy", "requests": "Requests", "pydantic": "Pydantic", "matplotlib": "Matplotlib",
		"jupyter": "Jupyter", "notebook": "Jupyter", "langchain": "LangChain", "boto3": "AWS SDK for Python (Boto3)",
		"openai": "OpenAI API", "anthropic": "Anthropic API", "transformers": "Hugging Face Transformers", "keras": "Keras",
		"djangorestframework": "Django REST framework", "alembic": "Alembic", "gunicorn": "Gunicorn", "uvicorn": "Uvicorn",
		"black": "Black", "ruff": "Ruff", "mypy": "mypy", "flake8": "Flake8", "airflow": "Apache Airflow",
		"apache-airflow": "Apache Airflow", "pyspark": "Apache Spark", "streamlit": "Streamlit", "polars": "Polars",
	},
	"go": {
		"github.com/gin-gonic/gin": "Gin", "github.com/labstack/echo/*": "Echo", "gorm.io/gorm": "GORM",
		"github.com/spf13/cobra": "Cobra", "github.com/stretchr/testify": "Testify", "github.com/gofiber/fiber/*": "Fiber",
		"google.golang.org/grpc": "gRPC", "github.com/prometheus/client_golang": "Prometheus",
		"k8s.io/client-go": "Kubernetes", "github.com/aws/aws-sdk-go-v2": "AWS SDK for Go",
	},
	"maven": {
		"org.springframework.boot:*": "Spring Boot", "org.springframework:*": "Spring Framework", "org.hibernate:*": "Hibernate",
		"org.hibernate.orm:*": "Hibernate", "junit:junit": "JUnit", "org.junit.jupiter:*": "JUnit", "org.mockito:*": "Mockito",
		"org.projectlombok:lombok": "Lombok", "com.fasterxml.jackson.core:*": "Jackson", "org.apache.kafka:*": "Apache Kafka",
		"io.quarkus:*": "Quarkus", "io.micronaut:*": "Micronaut", "org.springframework.cloud:*": "Spring Cloud",
		"org.springframework.security:*": "Spring Security", "org.apache.camel:*": "Apache Camel", "org.liquibase:*": "Liquibase",
		"org.flywaydb:*": "Flyway", "org.testcontainers:*": "Testcontainers", "io.rest-assured:*": "REST Assured",
		"org.seleniumhq.selenium:*": "Selenium", "org.apache.logging.log4j:*": "Log4j", "ch.qos.logback:*": "Logback",
	},
	"gradle-plugin": {
		"org.springframework.boot": "Spring Boot", "org.jetbrains.kotlin.jvm": "Kotlin", "com.android.application": "Android",
		"org.jetbrains.kotlin.android": "Kotlin", "io.quarkus": "Quarkus",
	},
	"nuget": {
		"microsoft.aspnetcore.*": "ASP.NET Core", "microsoft.entityframeworkcore*": "Entity Framework Core", "xunit": "xUnit.net",
		"nunit": "NUnit", "newtonsoft.json": "Json.NET", "serilog*": "Serilog", "automapper*": "AutoMapper", "moq": "Moq",
		"mediatr": "MediatR", "dapper": "Dapper", "fluentvalidation*": "FluentValidation", "masstransit*": "MassTransit",
		"polly": "Polly", "swashbuckle.aspnetcore*": "Swagger",
	},
	"rubygems": {
		"rails": "Ruby on Rails", "rspec": "RSpec", "rspec-rails": "RSpec", "sidekiq": "Sidekiq", "devise": "Devise",
		"puma": "Puma", "sinatra": "Sinatra", "capybara": "Capybara", "rubocop": "RuboCop", "pg": "PostgreSQL",
	},
	"cargo": {
		"tokio": "Tokio", "serde": "Serde", "actix-web": "Actix Web", "axum": "Axum", "rocket": "Rocket", "diesel": "Diesel",
		"sqlx": "SQLx", "clap": "clap", "reqwest": "reqwest",
	},
}

// genericParts say nothing about a product ("laravel/framework", "@angular/core"): the vendor names it then.
var genericParts = map[string]bool{
	"framework": true, "core": true, "cli": true, "sdk": true, "client": true, "bundle": true, "library": true, "lib": true,
	"php": true, "api": true, "common": true, "base": true, "server": true, "main": true,
}

var majorSuffix = regexp.MustCompile(`^v\d+$`)

// displayName is the name people write for a package: a known one, else one made from the package.
func displayName(ecosystem, pkg string) string {
	known := knownNames[ecosystem]

	if name, ok := known[pkg]; ok {
		return name
	}

	// The longest prefix wins (a map's order is random).
	best, bestLength := "", -1

	for key, name := range known {
		if prefix, ok := strings.CutSuffix(key, "*"); ok && strings.HasPrefix(pkg, prefix) && len(prefix) > bestLength {
			best, bestLength = name, len(prefix)
		}
	}

	if bestLength >= 0 {
		return best
	}

	return fallbackName(pkg)
}

// fallbackName is a package's own part ("guzzlehttp/guzzle" → "guzzle", "github.com/x/y/v2" → "y",
// "org.slf4j:slf4j-api" → "slf4j-api"), its vendor when that part is generic; a scoped npm package's scope
// ("@tiptap/pm" → "tiptap", "@tailwindcss/vite" → "tailwindcss": its plugins and parts belong to the scope's product).
func fallbackName(pkg string) string {
	if scope, _, ok := strings.Cut(strings.TrimPrefix(pkg, "@"), "/"); ok && strings.HasPrefix(pkg, "@") {
		return scope
	}

	parts := strings.FieldsFunc(pkg, func(r rune) bool { return r == '/' || r == ':' })

	for len(parts) > 1 && majorSuffix.MatchString(parts[len(parts)-1]) {
		parts = parts[:len(parts)-1]
	}

	if len(parts) == 0 {
		return pkg
	}

	last := parts[len(parts)-1]

	if genericParts[last] && len(parts) > 1 {
		return parts[len(parts)-2]
	}

	return last
}
