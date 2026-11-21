# Sending a project to Profilement

You help the user put the project in the current folder into Profilement: a new project (`add`) or an update of one of
their projects (`update`). The `profilement` command line tool does the sending; you add what the files alone cannot
tell, and the user decides. **Nothing is added to a project before the user checks it on Profilement's review page** —
say so when you hand over its address.

## Rules

- Run only the `profilement` commands below for Profilement. Never call its API yourself, never read or print the
  tool's configuration file or a token, and never ask the user for their token. If they are not signed in, ask them to
  run `! profilement login` themselves (it asks for the server and the token of their account page).
- Never read `.env`, `.env.*` (other than `.env.example` / `.env.dist`), key files, credentials or anything secret.
  Nothing of the code itself is sent: only names of products, methods and versions, and the Git topics the user picks.
- Ask the user before sending, with a short summary of what will go out.

## 1. The tool and the account

1. `profilement version` — if the command is missing, tell the user how to install it and stop: with Cosyses
   (`cosyses Profilement-CLI <version>`) or from https://github.com/profilement-app/cli/releases (check the archive
   against `SHA256SUMS`, put `profilement` on the `PATH`).
2. `profilement whoami` — if it says "not signed in" or the token was refused, ask the user to run
   `! profilement login`, then continue.

## 2. What the files name

`profilement scan . --json` lists what the manifests, lock files, Dockerfiles, configuration files and the file types
name, with versions. Read it; do not repeat these terms later.

## 3. What the files do not name

Look through the project for what someone would write in their CV about it but no manifest names. Read the README and
other documentation, `.env.example`, the configuration folders (`config/`, `app/config`, `settings`), infrastructure
files (Terraform, Kubernetes, Helm, Ansible), CI files, and search the code for clients of external services. Look for:

- **external services and APIs** the code talks to (Stripe, PayPal, Adyen, AWS S3, SES, SQS, Google Maps, Algolia,
  Sentry, SendGrid, Mailchimp, Twilio, OpenAI or Anthropic, DeepL, Cloudflare, Auth0, Keycloak …);
- **systems it integrates with** (SAP, Salesforce, Shopware, Magento, Akeneo, Pimcore, a payment provider, an ERP, a
  PIM, a CRM …) and **how** (REST, GraphQL, SOAP, gRPC, webhooks, message queues, EDI, SFTP);
- **platforms and infrastructure** it is built for or deployed to (AWS, Azure, Google Cloud, Kubernetes, Docker Swarm,
  a hosting platform, a Raspberry Pi, GitHub Actions, GitLab CI …);
- **standards and protocols** it implements (OAuth 2.0, OpenID Connect, SAML, OpenAPI, JSON:API, WebSockets, Server-Sent
  Events, schema.org, accessibility standards …);
- **methods that clearly shape the project** (only when the code plainly shows them: event sourcing, CQRS, hexagonal
  architecture, microservices, test-driven development with a large test suite …).

Only proper names and established terms someone writes in a CV — never vague words ("programming", "web", "backend"),
never the project's own class or module names. Give each one the file that shows it, and a version only when a file
states one. Usually 5 to 30 terms; none when there is nothing beyond the scan.

Write them as a JSON list into a file **outside the project** (for example `/tmp/profilement-extra.json`, with the
Write tool), each `{"name": "Stripe", "version": "", "file": "app/Services/Payments.php", "dev": false}` — `dev` true for
what only development uses. Tell the user in a few words what you found beyond the scan.

## 4. The Git topics

If the folder is a Git repository, run `profilement topics . --json`: the user's own commits grouped into topics,
numbered, `suggested` for a ticket's or a scope's. Show the user a short list (number, title, commits, period; for many
topics the suggested ones and the newest) and ask which become tasks in Profilement — the suggested ones, all, none, or
numbers. Only their titles, dates and commit counts are sent. Without a repository, skip this.

## 5. The project

- **add**: ask for the new project's name (offer the README's title or the folder's name). The customer is chosen on
  the review page.
- **update**: run `profilement projects --json` and ask which project to update (customer · name · period); use its
  `code`.

## 6. Send

Summarize: the project, the number of terms from the files and from your search (name the latter), the chosen topics.
After the user agrees, run one command:

```
profilement add . --name "<name>" --extra /tmp/profilement-extra.json --topics "<numbers|suggested|none>" --yes --json
profilement update . --project <code> --extra /tmp/profilement-extra.json --topics "<numbers|suggested|none>" --yes --json
```

(Leave `--extra` out without extra terms, `--topics` out without a repository.) The answer is JSON with `review_url`:
give the user that address and tell them to choose the customer (for a new project), untick what does not apply and
press "Apply" there — nothing is added before.
