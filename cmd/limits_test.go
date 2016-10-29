package cmd

import (
	"bytes"
	"fmt"
	"net/http"
	"testing"

	"github.com/arschles/assert"
	"github.com/deis/controller-sdk-go/api"
	"github.com/deis/workflow-cli/pkg/testutil"
)

type parseLimitCase struct {
	Input         string
	Key           string
	Value         string
	ExpectedError bool
	ExpectedMsg   string
}

func TestParseLimit(t *testing.T) {
	t.Parallel()

	var errorHint = ` doesn't fit format type=#unit or type=# or type=#/# or type=-/#
Examples: web=2G worker=500M db=300M db=1G/2G cmd=-/4G
          web=2 worker=500m db=300m db=1/2 cmd=-/4`

	cases := []parseLimitCase{
		{"web=2G", "web", "2G", false, ""},
		{"web=2", "web", "2", false, ""},
		{"web=100m", "web", "100m", false, ""},
		{"web=0.1", "web", "0.1", false, ""},
		{"web=.123", "web", ".123", false, ""},
		{"web=2G/4G", "web", "2G/4G", false, ""},
		{"web=2/4", "web", "2/4", false, ""},
		{"web=200m/400m", "web", "200m/400m", false, ""},
		{"web=0.2/0.4", "web", "0.2/0.4", false, ""},
		{"web=.2/.4", "web", ".2/.4", false, ""},
		{"web=-/4G", "web", "-/4G", false, ""},
		{"web=-/4", "web", "-/4", false, ""},
		{"web=-/400m", "web", "-/400m", false, ""},
		{"web=-/0.4", "web", "-/0.4", false, ""},
		{"web=-/.4", "web", "-/.4", false, ""},
		{"=1", "", "", true, "=1" + errorHint},
		{"web=", "", "", true, "web=" + errorHint},
		{"1=", "", "", true, "1=" + errorHint},
		{"web=G", "", "", true, "web=G" + errorHint},
		{"web=/", "", "", true, "web=/" + errorHint},
		{"web=/1", "", "", true, "web=/1" + errorHint},
	}

	for _, check := range cases {
		key, value, err := parseLimit(check.Input)
		if check.ExpectedError {
			assert.Equal(t, err.Error(), check.ExpectedMsg, "error")
		} else {
			assert.NoErr(t, err)
			assert.Equal(t, key, check.Key, "key")
			assert.Equal(t, value, check.Value, "value")
		}
	}
}

type parseLimitsCase struct {
	Input         []string
	ExpectedMap   map[string]interface{}
	ExpectedError bool
	ExpectedMsg   string
}

func TestLimitTags(t *testing.T) {
	t.Parallel()

	cases := []parseLimitsCase{
		{[]string{"web=1G", "worker=2"}, map[string]interface{}{"web": "1G", "worker": "2"}, false, ""},
		{[]string{"foo=", "web=1G"}, nil, true, `foo= doesn't fit format type=#unit or type=# or type=#/# or type=-/#
Examples: web=2G worker=500M db=300M db=1G/2G cmd=-/4G
          web=2 worker=500m db=300m db=1/2 cmd=-/4`},
	}

	for _, check := range cases {
		actual, err := parseLimits(check.Input)
		if check.ExpectedError {
			assert.Equal(t, err.Error(), check.ExpectedMsg, "error")
		} else {
			assert.NoErr(t, err)
			assert.Equal(t, actual, check.ExpectedMap, "map")
		}
	}
}

func TestLimitsList(t *testing.T) {
	t.Parallel()
	cf, server, err := testutil.NewTestServerAndClient()
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	server.Mux.HandleFunc("/v2/apps/enterprise/config/", func(w http.ResponseWriter, r *http.Request) {
		testutil.SetHeaders(w)
		fmt.Fprintf(w, `{
			"owner": "jkirk",
			"app": "enterprise",
			"values": {},
			"memory": {
				"web": "2G",
				"db": "-/1500M"
			},
			"cpu": {
				"web": "2",
				"worker": "1",
				"db": "500m/2000m"
			},
			"tags": {},
			"registry": {},
			"created": "2014-01-01T00:00:00UTC",
			"updated": "2014-01-01T00:00:00UTC",
			"uuid": "de1bf5b5-4a72-4f94-a10c-d2a3741cdf75"
		}`)
	})

	var b bytes.Buffer
	cmdr := DeisCmd{WOut: &b, ConfigFile: cf}

	err = cmdr.LimitsList("enterprise")
	assert.NoErr(t, err)
	assert.Equal(t, b.String(), `=== enterprise Limits

--- Memory
db      -/1500M
web     2G

--- CPU
db         500m/2000m
web        2
worker     1
`, "output")

	server.Mux.HandleFunc("/v2/apps/franklin/config/", func(w http.ResponseWriter, r *http.Request) {
		testutil.SetHeaders(w)
		fmt.Fprintf(w, `{
			"owner": "bedison",
			"app": "franklin",
			"values": {},
			"memory": {},
			"cpu": {},
			"tags": {},
			"registry": {},
			"created": "2014-01-01T00:00:00UTC",
			"updated": "2014-01-01T00:00:00UTC",
			"uuid": "de1bf5b5-4a72-4f94-a10c-d2a3741cdf75"
			}`)
	})
	b.Reset()

	err = cmdr.LimitsList("franklin")
	assert.NoErr(t, err)
	assert.Equal(t, b.String(), `=== franklin Limits

--- Memory
Unlimited

--- CPU
Unlimited
`, "output")
}

func TestLimitsSet(t *testing.T) {
	t.Parallel()
	cf, server, err := testutil.NewTestServerAndClient()
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	server.Mux.HandleFunc("/v2/apps/foo/config/", func(w http.ResponseWriter, r *http.Request) {
		testutil.SetHeaders(w)
		if r.Method == "POST" {
			testutil.AssertBody(t, api.Config{
				CPU: map[string]interface{}{
					"web": "100m",
				},
			}, r)
		}

		fmt.Fprintf(w, `{
			"owner": "jkirk",
			"app": "foo",
			"values": {},
			"memory": {},
			"cpu": {
				"web": "100m"
			},
			"tags": {},
			"registry": {},
			"created": "2014-01-01T00:00:00UTC",
			"updated": "2014-01-01T00:00:00UTC",
			"uuid": "de1bf5b5-4a72-4f94-a10c-d2a3741cdf75"
		}`)
	})

	var b bytes.Buffer
	cmdr := DeisCmd{WOut: &b, ConfigFile: cf}

	err = cmdr.LimitsSet("foo", []string{"web=100m"}, "cpu")
	assert.NoErr(t, err)

	assert.Equal(t, testutil.StripProgress(b.String()), `Applying limits... done

=== foo Limits

--- Memory
Unlimited

--- CPU
web     100m
`, "output")

	server.Mux.HandleFunc("/v2/apps/franklin/config/", func(w http.ResponseWriter, r *http.Request) {
		testutil.SetHeaders(w)
		if r.Method == "POST" {
			testutil.AssertBody(t, api.Config{
				Memory: map[string]interface{}{
					"web": "1G",
				},
			}, r)
		}

		fmt.Fprintf(w, `{
			"owner": "bedison",
			"app": "franklin",
			"values": {},
			"memory": {
				"web": "1G"
			},
			"cpu": {},
			"tags": {},
			"registry": {},
			"created": "2014-01-01T00:00:00UTC",
			"updated": "2014-01-01T00:00:00UTC",
			"uuid": "de1bf5b5-4a72-4f94-a10c-d2a3741cdf75"
		}`)
	})
	b.Reset()

	err = cmdr.LimitsSet("franklin", []string{"web=1G"}, "memory")
	assert.NoErr(t, err)

	assert.Equal(t, testutil.StripProgress(b.String()), `Applying limits... done

=== franklin Limits

--- Memory
web     1G

--- CPU
Unlimited
`, "output")

	// with requests/limit parameter
	server.Mux.HandleFunc("/v2/apps/jim/config/", func(w http.ResponseWriter, r *http.Request) {
		testutil.SetHeaders(w)
		if r.Method == "POST" {
			testutil.AssertBody(t, api.Config{
				Memory: map[string]interface{}{
					"web": "1G/2G",
					"worker": "-/3G",
					"cmd": nil,
					"db": nil,
				},
			}, r)
		}

		fmt.Fprintf(w, `{
			"owner": "foo",
			"app": "jim",
			"values": {},
			"memory": {
				"web": "1G/2G",
				"worker": "-/3G"
			},
			"cpu": {},
			"tags": {},
			"registry": {},
			"created": "2014-01-01T00:00:00UTC",
			"updated": "2014-01-01T00:00:00UTC",
			"uuid": "de1bf5b5-4a72-4f94-a10c-d2a3741cdf75"
		}`)
	})
	b.Reset()

	err = cmdr.LimitsSet("jim", []string{"web=1G/2G", "worker=-/3G", "cmd=-", "db=-/-"}, "memory")
	assert.NoErr(t, err)

	assert.Equal(t, testutil.StripProgress(b.String()), `Applying limits... done

=== jim Limits

--- Memory
web        1G/2G
worker     -/3G

--- CPU
Unlimited
`, "output")

	// with requests/limit parameter
	server.Mux.HandleFunc("/v2/apps/phew/config/", func(w http.ResponseWriter, r *http.Request) {
		testutil.SetHeaders(w)
		if r.Method == "POST" {
			testutil.AssertBody(t, api.Config{
				CPU: map[string]interface{}{
					"web": "1/2",
					"worker": "-/300m",
					"cmd": nil,
					"db": nil,
				},
			}, r)
		}

		fmt.Fprintf(w, `{
			"owner": "bar",
			"app": "phew",
			"values": {},
			"cpu": {
				"web": "1/2",
				"worker": "-/300m"
			},
			"cpu": {},
			"tags": {},
			"registry": {},
			"created": "2014-01-01T00:00:00UTC",
			"updated": "2014-01-01T00:00:00UTC",
			"uuid": "de1bf5b5-4a72-4f94-a10c-d2a3741cdf75"
		}`)
	})
	b.Reset()

	err = cmdr.LimitsSet("phew", []string{"web=1/2", "worker=-/300m", "cmd=-", "db=-/-"}, "cpu")
	assert.NoErr(t, err)

	assert.Equal(t, testutil.StripProgress(b.String()), `Applying limits... done

=== phew Limits

--- Memory
Unlimited

--- CPU
web        1/2
worker     -/300m
`, "output")
}

func TestLimitsUnset(t *testing.T) {
	t.Parallel()
	cf, server, err := testutil.NewTestServerAndClient()
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	server.Mux.HandleFunc("/v2/apps/foo/config/", func(w http.ResponseWriter, r *http.Request) {
		testutil.SetHeaders(w)
		if r.Method == "POST" {
			testutil.AssertBody(t, api.Config{
				Memory: map[string]interface{}{
					"web": nil,
				},
			}, r)
		}

		fmt.Fprintf(w, `{
			"owner": "jkirk",
			"app": "foo",
			"values": {},
			"memory": {},
			"cpu": {
				"web": "100m"
			},
			"tags": {},
			"registry": {},
			"created": "2014-01-01T00:00:00UTC",
			"updated": "2014-01-01T00:00:00UTC",
			"uuid": "de1bf5b5-4a72-4f94-a10c-d2a3741cdf75"
		}`)
	})

	var b bytes.Buffer
	cmdr := DeisCmd{WOut: &b, ConfigFile: cf}

	err = cmdr.LimitsUnset("foo", []string{"web"}, "memory")
	assert.NoErr(t, err)

	assert.Equal(t, testutil.StripProgress(b.String()), `Applying limits... done

=== foo Limits

--- Memory
Unlimited

--- CPU
web     100m
`, "output")

	server.Mux.HandleFunc("/v2/apps/franklin/config/", func(w http.ResponseWriter, r *http.Request) {
		testutil.SetHeaders(w)
		if r.Method == "POST" {
			testutil.AssertBody(t, api.Config{
				CPU: map[string]interface{}{
					"web": nil,
				},
			}, r)
		}

		fmt.Fprintf(w, `{
			"owner": "bedison",
			"app": "franklin",
			"values": {},
			"memory": {
				"web": "1G"
			},
			"cpu": {},
			"tags": {},
			"registry": {},
			"created": "2014-01-01T00:00:00UTC",
			"updated": "2014-01-01T00:00:00UTC",
			"uuid": "de1bf5b5-4a72-4f94-a10c-d2a3741cdf75"
		}`)
	})
	b.Reset()

	err = cmdr.LimitsUnset("franklin", []string{"web"}, "cpu")
	assert.NoErr(t, err)

	assert.Equal(t, testutil.StripProgress(b.String()), `Applying limits... done

=== franklin Limits

--- Memory
web     1G

--- CPU
Unlimited
`, "output")
}
