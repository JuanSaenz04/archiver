package crawler

import (
	"context"
	"errors"
	"os/exec"
	"testing"
	"uuid"

	"github.com/JuanSaenz04/archiver/internal/joblogs"
	"github.com/JuanSaenz04/archiver/internal/models"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestCrawlerCapturesProcessOutputAndResult(t *testing.T) {
	for _, failed := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "failure"}[failed], func(t *testing.T) {
			mr := miniredis.RunT(t)
			rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
			defer rdb.Close()
			logs := joblogs.NewStore(rdb)
			crawler := NewCrawler(Config{Logs: logs, AnubisMode: AnubisModeOff}, newTestStore(t))
			crawler.runCmd = func(original *exec.Cmd) error {
				cmd := exec.Command("sh", "-c", "printf 'page captured\\n'; printf 'warning\\n' >&2")
				cmd.Stdout, cmd.Stderr = original.Stdout, original.Stderr
				if err := cmd.Run(); err != nil {
					return err
				}
				if failed {
					return errors.New("crawl failed")
				}
				return nil
			}
			id := uuid.New()
			err := crawler.Run(context.Background(), id.String(), models.Archive{ID: id, SourceURL: "https://example.com"}, models.CrawlOptions{})
			if failed {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			entries, err := logs.Read(context.Background(), id.String(), "")
			require.NoError(t, err)
			var stdout, stderr bool
			for _, e := range entries {
				stdout = stdout || e.Source == "stdout" && e.Text == "page captured"
				stderr = stderr || e.Source == "stderr" && e.Text == "warning"
			}
			require.True(t, stdout)
			require.True(t, stderr)
			if failed {
				require.Contains(t, entries[len(entries)-1].Text, "Job failed")
			} else {
				require.Equal(t, "Job completed", entries[len(entries)-1].Text)
			}
		})
	}
}
