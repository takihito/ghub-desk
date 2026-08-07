package cmd

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"ghub-desk/ghubclient"
	"ghub-desk/session"
	"ghub-desk/store"
)

// PullCmd represents the pull command structure
type PullCmd struct {
	CommonTargetOptions `embed:""`

	// Options
	NoStore      bool          `name:"no-store" help:"Do not save to local SQLite database"`
	Stdout       bool          `name:"stdout" help:"Print API response to stdout"`
	IntervalTime time.Duration `help:"Sleep interval between API requests" default:"3s"`
}

// Run implements the pull command execution
func (p *PullCmd) Run(cli *CLI) error {
	// Determine target from flags
	target, err := p.CommonTargetOptions.GetTarget()
	if err != nil {
		return err
	}

	storeData := !p.NoStore
	cli.debugf("DEBUG: Pulling target='%s', store=%v, stdout=%v, interval=%v\n", target, storeData, p.Stdout, p.IntervalTime)

	// Load configuration once via CLI helper
	cfg, err := cli.Config()
	if err != nil {
		return fmt.Errorf("configuration error: %w", err)
	}
	if cfg.DatabasePath != "" {
		store.SetDBPath(cfg.DatabasePath)
	}
	session.SetPath(cfg.SessionPath)

	// Initialize GitHub client
	client, err := ghubclient.InitClient(cfg)
	if err != nil {
		return fmt.Errorf("github client initialization error: %w", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	signalSeen := make(chan os.Signal, 1)
	go func() {
		select {
		case s := <-sigChan:
			signalSeen <- s
			cancel()
		case <-ctx.Done():
		}
	}()

	var db *sql.DB
	if storeData || target == "all-teams-users" || target == "all-repos-teams" || target == "all-repos-users" {
		db, err = store.Connect()
		if err != nil {
			return fmt.Errorf("failed to connect to database: %w", err)
		}
		defer db.Close()
	}

	req := ghubclient.TargetRequest{Kind: target}
	switch target {
	case "team-user":
		if err := validateTeamName(p.TeamUser); err != nil {
			return err
		}
		req.TeamSlug = p.TeamUser
	case "repos-users":
		if err := validateRepoName(p.RepoUsers); err != nil {
			return err
		}
		req.RepoName = p.RepoUsers
	case "repos-teams":
		if err := validateRepoName(p.RepoTeams); err != nil {
			return err
		}
		req.RepoName = p.RepoTeams
	case "user-repos":
		if err := validateUserLogin(p.UserRepos); err != nil {
			return err
		}
		return fmt.Errorf("--user-repos is not available for the pull command. Please specify --user-repos with the view command")
	case "user":
		if err := validateUserLogin(p.User); err != nil {
			return err
		}
		return fmt.Errorf("--user is not available for the pull command. Please specify --user with the view command")
	case "user-teams":
		if err := validateUserLogin(p.UserTeams); err != nil {
			return err
		}
		return fmt.Errorf("--user-teams is not available for the pull command. Please specify --user-teams with the view command")
	case "team-repos":
		if err := validateTeamName(p.TeamRepos); err != nil {
			return err
		}
		return fmt.Errorf("--team-repos is not available for the pull command. Please specify --team-repos with the view command")
	}
	sessionKey := buildPullSessionKey(target, req, storeData, p.Stdout, p.IntervalTime)
	pullSession, err := session.LoadPull(sessionKey)
	resuming := err == nil
	if err != nil && !errors.Is(err, session.ErrNotFound) {
		return fmt.Errorf("failed to load session: %w", err)
	}
	expectedInterval := p.IntervalTime
	if resuming {
		storedInterval, parseErr := time.ParseDuration(pullSession.Interval)
		if parseErr != nil {
			fmt.Printf("Invalid interval value (%q) in existing session, starting a new session: %v\n", pullSession.Interval, parseErr)
			resuming = false
		}
		if resuming {
			if pullSession.Target != target ||
				pullSession.Store != storeData ||
				pullSession.Stdout != p.Stdout ||
				storedInterval != expectedInterval ||
				(pullSession.TeamSlug != "" && pullSession.TeamSlug != req.TeamSlug) ||
				(pullSession.RepoName != "" && pullSession.RepoName != req.RepoName) {
				fmt.Println("Existing session options differ from current options, starting a new session.")
				resuming = false
			}
		}
	}
	if !resuming {
		pullSession = session.NewPullSession(sessionKey, target)
		pullSession.Store = storeData
		pullSession.Stdout = p.Stdout
		pullSession.Interval = expectedInterval.String()
		pullSession.TeamSlug = req.TeamSlug
		pullSession.RepoName = req.RepoName
		if err := session.SavePull(pullSession); err != nil {
			return fmt.Errorf("failed to initialize session: %w", err)
		}
	} else {
		fmt.Printf("Resuming previous pull session (endpoint=%s, last page=%d, items fetched so far=%d)\n",
			pullSession.Endpoint, pullSession.LastPage, pullSession.FetchedCount)
	}

	recorder := session.NewProgressRecorder(pullSession)
	pullOptions := ghubclient.PullOptions{
		Store:    storeData,
		Stdout:   p.Stdout,
		Interval: p.IntervalTime,
		Resume: ghubclient.ResumeState{
			Endpoint: pullSession.Endpoint,
			Metadata: pullSession.Metadata,
			LastPage: pullSession.LastPage,
			Count:    pullSession.FetchedCount,
		},
		Progress: recorder,
	}

	err = ghubclient.HandlePullTarget(
		ctx,
		client,
		db,
		cfg.Organization,
		req,
		pullOptions,
	)

	var receivedSignal os.Signal
	select {
	case receivedSignal = <-signalSeen:
	default:
	}

	if errors.Is(err, context.Canceled) {
		printInterruptionSummary(receivedSignal, pullSession)
		return nil
	}

	if err != nil {
		return err
	}

	if err := session.RemovePull(sessionKey); err != nil && !errors.Is(err, session.ErrNotFound) {
		return fmt.Errorf("failed to remove session: %w", err)
	}

	return nil
}

func buildPullSessionKey(target string, req ghubclient.TargetRequest, store bool, stdout bool, interval time.Duration) string {
	parts := []string{target}
	if req.TeamSlug != "" {
		parts = append(parts, "team:"+req.TeamSlug)
	}
	if req.RepoName != "" {
		parts = append(parts, "repo:"+req.RepoName)
	}
	if req.UserLogin != "" {
		parts = append(parts, "user:"+req.UserLogin)
	}
	parts = append(parts,
		fmt.Sprintf("store:%t", store),
		fmt.Sprintf("stdout:%t", stdout),
		fmt.Sprintf("interval:%s", interval))
	return strings.Join(parts, "|")
}

func printInterruptionSummary(sig os.Signal, sess *session.PullSession) {
	reason := "context canceled"
	if sig != nil {
		reason = sig.String()
	}
	fmt.Printf("INFO: Pull interrupted after receiving %s.\n", reason)
	fmt.Printf("      endpoint=%s, last page=%d, items fetched so far=%d\n", sess.Endpoint, sess.LastPage, sess.FetchedCount)
	if len(sess.Metadata) > 0 {
		fmt.Printf("      metadata: %v\n", sess.Metadata)
	}
	fmt.Printf("      Interruption state saved to %s.\n", session.Path())
}
