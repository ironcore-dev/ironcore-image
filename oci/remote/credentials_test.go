// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package remote

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestRemote(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Remote Suite")
}

func writeDockerConfig(dir string, content string) string {
	path := filepath.Join(dir, "config.json")
	ExpectWithOffset(1, os.WriteFile(path, []byte(content), 0600)).To(Succeed())
	return path
}

func basicAuth(user, pass string) string {
	return base64.StdEncoding.EncodeToString([]byte(user + ":" + pass))
}

var _ = Describe("DockerCredentialFunc", func() {
	var tmpDir string

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "credentials-test-*")
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { _ = os.RemoveAll(tmpDir) })
	})

	It("should return credentials from a config file", func() {
		configPath := writeDockerConfig(tmpDir, `{
			"auths": {
				"registry.example.com": {
					"auth": "`+basicAuth("user1", "pass1")+`"
				}
			}
		}`)

		credFunc, err := DockerCredentialFunc(configPath)
		Expect(err).NotTo(HaveOccurred())

		user, pass, err := credFunc("registry.example.com")
		Expect(err).NotTo(HaveOccurred())
		Expect(user).To(Equal("user1"))
		Expect(pass).To(Equal("pass1"))
	})

	It("should return empty credentials for an unknown host", func() {
		configPath := writeDockerConfig(tmpDir, `{
			"auths": {
				"registry.example.com": {
					"auth": "`+basicAuth("user1", "pass1")+`"
				}
			}
		}`)

		credFunc, err := DockerCredentialFunc(configPath)
		Expect(err).NotTo(HaveOccurred())

		user, pass, err := credFunc("unknown.example.com")
		Expect(err).NotTo(HaveOccurred())
		Expect(user).To(BeEmpty())
		Expect(pass).To(BeEmpty())
	})

	It("should use the default Docker config when no path is provided", func() {
		configDir := filepath.Join(tmpDir, "docker")
		Expect(os.Mkdir(configDir, 0700)).To(Succeed())
		writeDockerConfig(configDir, `{
			"auths": {
				"default.example.com": {
					"auth": "`+basicAuth("defaultuser", "defaultpass")+`"
				}
			}
		}`)

		origEnv := os.Getenv("DOCKER_CONFIG")
		Expect(os.Setenv("DOCKER_CONFIG", configDir)).To(Succeed())
		DeferCleanup(func() { _ = os.Setenv("DOCKER_CONFIG", origEnv) })

		credFunc, err := DockerCredentialFunc("")
		Expect(err).NotTo(HaveOccurred())

		user, pass, err := credFunc("default.example.com")
		Expect(err).NotTo(HaveOccurred())
		Expect(user).To(Equal("defaultuser"))
		Expect(pass).To(Equal("defaultpass"))
	})

	It("should return the refresh token when username and password are empty", func() {
		configPath := writeDockerConfig(tmpDir, `{
			"auths": {
				"token.example.com": {
					"identitytoken": "my-refresh-token"
				}
			}
		}`)

		credFunc, err := DockerCredentialFunc(configPath)
		Expect(err).NotTo(HaveOccurred())

		user, secret, err := credFunc("token.example.com")
		Expect(err).NotTo(HaveOccurred())
		Expect(user).To(BeEmpty())
		Expect(secret).To(Equal("my-refresh-token"))
	})

	It("should return empty credentials from an empty config file", func() {
		configPath := writeDockerConfig(tmpDir, `{"auths": {}}`)

		credFunc, err := DockerCredentialFunc(configPath)
		Expect(err).NotTo(HaveOccurred())

		user, pass, err := credFunc("any.example.com")
		Expect(err).NotTo(HaveOccurred())
		Expect(user).To(BeEmpty())
		Expect(pass).To(BeEmpty())
	})
})
