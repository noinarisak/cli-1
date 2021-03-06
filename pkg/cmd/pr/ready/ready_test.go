package ready

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"regexp"
	"testing"

	"github.com/cli/cli/context"
	"github.com/cli/cli/git"
	"github.com/cli/cli/internal/config"
	"github.com/cli/cli/internal/ghrepo"
	"github.com/cli/cli/pkg/cmdutil"
	"github.com/cli/cli/pkg/httpmock"
	"github.com/cli/cli/pkg/iostreams"
	"github.com/cli/cli/test"
	"github.com/google/shlex"
)

func runCommand(rt http.RoundTripper, isTTY bool, cli string) (*test.CmdOut, error) {
	io, _, stdout, stderr := iostreams.Test()
	io.SetStdoutTTY(isTTY)
	io.SetStdinTTY(isTTY)
	io.SetStderrTTY(isTTY)

	factory := &cmdutil.Factory{
		IOStreams: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{Transport: rt}, nil
		},
		Config: func() (config.Config, error) {
			return config.NewBlankConfig(), nil
		},
		BaseRepo: func() (ghrepo.Interface, error) {
			return ghrepo.New("OWNER", "REPO"), nil
		},
		Remotes: func() (context.Remotes, error) {
			return context.Remotes{
				{
					Remote: &git.Remote{Name: "origin"},
					Repo:   ghrepo.New("OWNER", "REPO"),
				},
			}, nil
		},
		Branch: func() (string, error) {
			return "main", nil
		},
	}

	cmd := NewCmdReady(factory, nil)

	argv, err := shlex.Split(cli)
	if err != nil {
		return nil, err
	}
	cmd.SetArgs(argv)

	cmd.SetIn(&bytes.Buffer{})
	cmd.SetOut(ioutil.Discard)
	cmd.SetErr(ioutil.Discard)

	_, err = cmd.ExecuteC()
	return &test.CmdOut{
		OutBuf: stdout,
		ErrBuf: stderr,
	}, err
}

func TestPRReady(t *testing.T) {
	http := &httpmock.Registry{}
	defer http.Verify(t)

	http.StubResponse(200, bytes.NewBufferString(`
	{ "data": { "repository": {
		"pullRequest": { "number": 444, "closed": false, "isDraft": true}
	} } }
	`))
	http.StubResponse(200, bytes.NewBufferString(`{"id": "THE-ID"}`))

	output, err := runCommand(http, true, "444")
	if err != nil {
		t.Fatalf("error running command `pr ready`: %v", err)
	}

	r := regexp.MustCompile(`Pull request #444 is marked as "ready for review"`)

	if !r.MatchString(output.Stderr()) {
		t.Fatalf("output did not match regexp /%s/\n> output\n%q\n", r, output.Stderr())
	}
}

func TestPRReady_alreadyReady(t *testing.T) {
	http := &httpmock.Registry{}
	defer http.Verify(t)

	http.StubResponse(200, bytes.NewBufferString(`
	{ "data": { "repository": {
		"pullRequest": { "number": 445, "closed": false, "isDraft": false}
	} } }
	`))

	output, err := runCommand(http, true, "445")
	if err != nil {
		t.Fatalf("error running command `pr ready`: %v", err)
	}

	r := regexp.MustCompile(`Pull request #445 is already "ready for review"`)

	if !r.MatchString(output.Stderr()) {
		t.Fatalf("output did not match regexp /%s/\n> output\n%q\n", r, output.Stderr())
	}
}

func TestPRReady_closed(t *testing.T) {
	http := &httpmock.Registry{}
	defer http.Verify(t)

	http.StubResponse(200, bytes.NewBufferString(`
	{ "data": { "repository": {
		"pullRequest": { "number": 446, "closed": true, "isDraft": true}
	} } }
	`))

	output, err := runCommand(http, true, "446")
	if err == nil {
		t.Fatalf("expected an error running command `pr ready` on a review that is closed!: %v", err)
	}

	r := regexp.MustCompile(`Pull request #446 is closed. Only draft pull requests can be marked as "ready for review"`)

	if !r.MatchString(output.Stderr()) {
		t.Fatalf("output did not match regexp /%s/\n> output\n%q\n", r, output.Stderr())
	}
}
