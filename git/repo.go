package git

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"

	log "github.com/Sirupsen/logrus"
)

// Repo represents a git repo, it contains its path and remote.
// This struct has methods to boostrap and update the git repository.
//
// This struct also implements the `command.Updater` interface.
type Repo struct {
	Path   string `json:"path"`
	Remote string `json:"remote"`

	name string
	head string

	postReceiveHooks []PostReceiveHook
}

// NewRepo initialize and returns a repository pointer.
func NewRepo(repoPath, remote string, postReceiveHooks ...PostReceiveHook) *Repo {
	return &Repo{
		name:             path.Base(repoPath),
		Path:             repoPath,
		Remote:           remote,
		postReceiveHooks: postReceiveHooks,
	}
}

type rawRepo Repo

// MarshalJSON implements the marshal json interface
func (r *Repo) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		*rawRepo
		Version string `json:"version"`
	}{
		rawRepo: (*rawRepo)(r),
		Version: r.CurrentHead(),
	})
}

// PostReceiveHook is a function that runs after
// clonning and updating the repo.
type PostReceiveHook func(*Repo) error

const (
	upstream = "origin/master"
)

// Update runs a git fetch to the `origin` remote,
// if the origin/master has a different sha that the
// current head, it executes a `git reset --hard origin/master`
// and then runs the repository post receive hooks.
// The function must return the new head sha if succeed.
// If no updates are found, it returns the actual head SHA.
func (r *Repo) Update() (updatedHeadSHA string, err error) {
	logger := r.logger()
	logger.Info("Updating...")

	fetch := exec.Command("git", "fetch", "origin")
	fetch.Dir = r.Path

	if err := fetch.Run(); err != nil {
		return "", err
	}

	originHeadCmd := exec.Command("git", "rev-parse", "--short", upstream)
	originHeadCmd.Dir = r.Path

	originHead, err := originHeadCmd.Output()
	if err != nil {
		return "", err
	}

	oHead := sanitizeOutput(originHead)
	if oHead == r.head {
		logger.Info("No new updates")
		return r.head, nil
	}
	logger.Info("Update found")

	updateCmd := exec.Command("git", "reset", "--hard", upstream)
	updateCmd.Dir = r.Path

	logger = logger.WithFields(log.Fields{"new_version": oHead})
	logger.Info("Downloading...")
	if err := updateCmd.Run(); err != nil {
		return "", err
	}

	if err := r.updateHead(); err != nil {
		return "", err
	}

	cleanCmd := exec.Command("git", "clean", "-f", "-d", "-X")
	cleanCmd.Dir = r.Path

	logger.Info("Cleaning...")
	if err := cleanCmd.Run(); err != nil {
		return "", err
	}
	logger.Info("Update finished")
	if err := r.runPostReceiveHooks(); err != nil {
		return "", err
	}

	return r.head, nil
}

// CurrentHead returns the head sha as a string.
func (r *Repo) CurrentHead() string {
	return r.head
}

func (r *Repo) runPostReceiveHooks() error {
	r.logger().Info("Aplying post-receive hooks")
	for _, hook := range r.postReceiveHooks {
		if err := hook(r); err != nil {
			return err
		}
	}

	return nil
}

// Bootstrap clones the repo if it not exists and runs the post-receive hooks.
// If there is no errors, it updates the repository current head sha. If the
// repo is already cloned, the function receives an arguments that indicates
// if we want to update (git pull) the repo or not.
func (r *Repo) Bootstrap(wantToUpdate bool) error {
	updated := false

	if _, err := os.Stat(fmt.Sprintf("%s/.git", r.Path)); err != nil {
		if os.IsExist(err) {
			return err
		}

		updated = true
		logger := r.logger()

		logger.Info("Clonning...")
		clone := exec.Command("git", "clone", "--single-branch", "--branch", "master", r.Remote, r.Path)
		clone.Dir = path.Dir(r.Path)

		if err := clone.Run(); err != nil {
			return err
		}

		if err := r.updateHead(); err != nil {
			return err
		}

		if err := r.runPostReceiveHooks(); err != nil {
			return err
		}
	}

	if err := r.updateHead(); err != nil {
		return err
	}

	if !updated && wantToUpdate {
		if _, err := r.Update(); err != nil {
			return err
		}
	}

	return nil
}

func (r *Repo) logger() *log.Entry {
	return log.WithFields(log.Fields{
		"process": r.name,
		"version": r.head,
		"repo":    r.Path,
	})
}

func (r *Repo) updateHead() error {
	headCmd := exec.Command("git", "log", "--pretty=format:%h", "-n", "1")
	headCmd.Dir = r.Path

	head, err := headCmd.Output()
	if err != nil {
		return err
	}

	r.head = sanitizeOutput(head)
	return nil
}

// AddPostReceiveHooks receives one or multiple PostReceiveHooks and appends
// them to the repo `postReceiveHooks` private attribute.
func (r *Repo) AddPostReceiveHooks(handlers ...PostReceiveHook) {
	r.postReceiveHooks = append(r.postReceiveHooks, handlers...)
}

func sanitizeOutput(b []byte) string {
	return string(bytes.TrimSpace(b))
}

// NpmInstallHook is a PostReceiveHook preset that runs a
// `npm install --production` command.
func NpmInstallHook(r *Repo) error {
	npmInstall := exec.Command("npm", "install", "--production")
	npmInstall.Dir = r.Path

	r.logger().Info("Running npm install")
	return npmInstall.Run()
}

// NpmPruneHook is a PostReceiveHook preset that runs a `npm prune` command.
func NpmPruneHook(r *Repo) error {
	prune := exec.Command("npm", "prune")
	prune.Dir = r.Path

	r.logger().Info("Running npm prune")
	return prune.Run()
}
