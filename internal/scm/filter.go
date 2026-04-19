package scm

import (
	"net/url"
	"os"
	"strings"
)

func hasMatchingTopic(rpTopics []string) bool {
	envTopics := strings.Split(os.Getenv("GHORG_TOPICS"), ",")

	// If user defined a list of topics, check if any match with this repo
	if os.Getenv("GHORG_TOPICS") != "" {
		for _, rpTopic := range rpTopics {
			for _, envTopic := range envTopics {
				if rpTopic == envTopic {
					return true
				}
			}
		}
		return false
	}

	// If no user defined topics are specified, accept any topics
	return true
}

// ReplaceSSHHostname replaces the hostname in an SSH clone URL with the custom hostname
// from GHORG_SSH_HOSTNAME. Only applies to SSH URLs (git@host:...).
func ReplaceSSHHostname(cloneURL string) string {
	sshHostname := os.Getenv("GHORG_SSH_HOSTNAME")
	if sshHostname == "" {
		return cloneURL
	}

	// Handle SSH URLs in the form git@hostname:path
	if strings.HasPrefix(cloneURL, "git@") {
		parts := strings.SplitN(cloneURL, ":", 2)
		if len(parts) == 2 {
			return "git@" + sshHostname + ":" + parts[1]
		}
	}

	// Handle SSH URLs in the form ssh://git@hostname/path
	if strings.HasPrefix(cloneURL, "ssh://") {
		parsed, err := url.Parse(cloneURL)
		if err != nil {
			return cloneURL
		}
		parsed.Host = sshHostname
		return parsed.String()
	}

	return cloneURL
}
