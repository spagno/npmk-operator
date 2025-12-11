# TODO

## GitHub Actions Automation

- [x] **CI Pipeline** - Build and test on every push/PR
  - Run `go test ./...`
  - Run `go vet ./...`
  - Run `helm lint charts/npmk-operator`
  - Build Docker image

- [x] **Release Workflow** - Automated release on tag push
  - Build and push Docker image to registry (GHCR)
  - Create GitHub Release with changelog
  - Publish Helm chart to GitHub Pages or OCI registry

- [x] **Security Scanning**
  - Add Trivy or Snyk for container vulnerability scanning
  - Add Go vulnerability scanning (`govulncheck`)

- [x] **Code Quality**
  - Add golangci-lint
  - Add codecov for test coverage reporting

## Future Enhancements

- [ ] Add controller tests with envtest
- [ ] Add e2e tests
- [x] Add Dependabot for dependency updates
- [ ] Add CODEOWNERS file
- [x] Add issue/PR templates
