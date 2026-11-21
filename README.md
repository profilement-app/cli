# Profilement CLI

`profilement` sends what a software project uses — its tools, frameworks, libraries and versions, and, if you choose,
topics of your own Git history as tasks — from the project's folder to [Profilement](https://www.profilement.com).
There you check everything on a review page and apply it to a new or an existing project. **Nothing is added before.**

It reads files only, no AI and no upload of code: the manifests and lock files (`composer.json`/`.lock`,
`package.json` with `package-lock.json`, `yarn.lock` or `pnpm-lock.yaml`, `go.mod`, `requirements*.txt`,
`pyproject.toml`, `Gemfile`/`.lock`, `pom.xml`, `build.gradle(.kts)`, `*.csproj`, `Cargo.toml`), the images of
Dockerfiles and Compose files, the configuration files of tools (PHPStan, ESLint, Vite, GitHub Actions, GitLab CI …),
and which languages the project's files are written in. Dependency folders (`vendor`, `node_modules` …) are skipped.

## Install

With [Cosyses](https://www.cosyses.app) (once released):

```
cosyses Profilement-CLI 0.1
```

Or download the archive for your system from the [releases](https://github.com/profilement-app/cli/releases), check it
against `SHA256SUMS`, and put `profilement` on your `PATH`.

## Use

1. In Profilement, open your account page and create a token in the card **Access tokens** (one per computer).
2. Connect this computer:

   ```
   profilement login            # asks for the server (default https://www.profilement.com) and the token
   ```

3. In a project's folder:

   ```
   profilement scan             # shows what would be sent; nothing is sent
   profilement add              # proposes a new project
   profilement update           # proposes an update of one of your projects (the folder's is remembered)
   ```

   `add` and `update` list your own commits (by `git config user.email`, on every branch, without merges) grouped
   into topics — a ticket like `SHOP-123`, an issue `#42`, a conventional commit's scope `feat(checkout): …`, else
   each commit's subject — and ask which become tasks. Topics with a ticket or scope are suggested (marked `*`).
   Only the topics you choose are sent: their title, dates and number of commits, never the commits themselves.

4. Open the printed review page, tick what applies, and apply it.

Options of `add` and `update`: `--name NAME` (add), `--project CODE` (update), `--no-git`, `--author EMAIL`, `--yes`
(ask nothing; the suggested topics). `profilement scan --json` prints the terms as JSON.

## Security

- The token is a personal access token of your Profilement account, limited to sending proposals and listing your
  projects. It is kept in your user's configuration folder (`profilement/config.json`, readable by you only) and can
  be deleted any time on your account page.
- Only `https` servers are accepted (`http` only on this computer), redirects are never followed, and the remote of
  your Git repository is sent without credentials.
- `$PROFILEMENT_URL`, `$PROFILEMENT_TOKEN` and `$PROFILEMENT_CONFIG` override the settings (for CI).

## Development

Go 1.27, standard library only (`cosyses Go 1.27`).

```
go vet ./...
go test ./...
go build -o build/profilement ./cmd/profilement
```

Every push and pull request is checked (`gofmt`, `go vet`, `go test`; `.github/workflows/check.yml`).

**Releases follow the app's version**: once a release of Profilement (`profilement-app/app`) has been deployed, its
workflow starts `.github/workflows/release.yml` here with its tag. It checks and tests `master` and, only if everything
passes, builds the binaries for Linux, macOS and Windows (amd64 and arm64) with `SHA256SUMS` and creates a release with
exactly the app's tag, so a CLI version always fits the server's API. A tag released already is skipped. By hand:
**Actions › Release › Run workflow** with the version.

## License

MIT, see [LICENSE](LICENSE).
