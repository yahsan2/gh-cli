package addsub

import (
	"net/http"
	"testing"

	"github.com/cli/cli/v2/internal/ghrepo"
	"github.com/cli/cli/v2/pkg/cmdutil"
	"github.com/cli/cli/v2/pkg/iostreams"
	"github.com/google/shlex"
	"github.com/stretchr/testify/assert"
)

func TestNewCmdAddSub(t *testing.T) {
	tests := []struct {
		name     string
		cli      string
		wants    AddSubOptions
		wantsErr bool
	}{
		{
			name: "basic usage",
			cli:  "123 456",
			wants: AddSubOptions{
				ParentIssueArg: "123",
				SubIssueArg:    "456",
			},
		},
		{
			name:     "no arguments",
			cli:      "",
			wantsErr: true,
		},
		{
			name:     "too many arguments",
			cli:      "123 456 789",
			wantsErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ios, _, _, _ := iostreams.Test()
			f := &cmdutil.Factory{
				IOStreams: ios,
			}

			var opts *AddSubOptions
			cmd := NewCmdAddSub(f, func(gotOpts *AddSubOptions) error {
				opts = gotOpts
				return nil
			})

			args, err := shlex.Split(tt.cli)
			assert.NoError(t, err)

			cmd.SetArgs(args)
			_, err = cmd.ExecuteC()

			if tt.wantsErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.wants.ParentIssueArg, opts.ParentIssueArg)
			assert.Equal(t, tt.wants.SubIssueArg, opts.SubIssueArg)
		})
	}
}

func TestAddSubRun_SameIssue(t *testing.T) {
	ios, _, _, _ := iostreams.Test()
	
	opts := &AddSubOptions{
		IO:             ios,
		ParentIssueArg: "123",
		SubIssueArg:    "123",
		HttpClient: func() (*http.Client, error) {
			return nil, nil // Won't be called for same issue check
		},
		BaseRepo: func() (ghrepo.Interface, error) {
			return nil, nil // Won't be called for same issue check
		},
	}

	err := addSubRun(opts)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "an issue cannot be its own sub-issue")
}