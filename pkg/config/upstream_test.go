package config

import (
	"fmt"
	"testing"

	assert2 "github.com/stretchr/testify/assert"
)

func TestHTTPStatusConfig_Is(t *testing.T) {
	assert := assert2.New(t)
	type testcase struct {
		received int
		cfg      *HTTPStatusConfig
		expected bool
	}

	testcases := []testcase{
		{received: 400, cfg: &HTTPStatusConfig{Exact: 400}, expected: true},
		{received: 401, cfg: &HTTPStatusConfig{Exact: 400}, expected: false},
		{received: 400, cfg: &HTTPStatusConfig{Range: "400-404"}, expected: true},
		{received: 400, cfg: &HTTPStatusConfig{Range: "500-600"}, expected: false},
	}

	for i, tc := range testcases {
		t.Run(fmt.Sprintf("case %d", i+1), func(t *testing.T) {
			assert.Equal(tc.expected, tc.cfg.Is(tc.received, ""))
		})
	}

	t.Run("invalid range", func(t *testing.T) {
		cfg := &HTTPStatusConfig{Range: "400-"}
		assert.False(cfg.Is(401, ""))
	})

	t.Run("except excludes from range", func(t *testing.T) {
		cfg := &HTTPStatusConfig{Range: "400-499", Except: []int{401, 403}}
		testCases := []struct {
			status   int
			expected bool
		}{
			{400, true},
			{401, false},
			{402, true},
			{403, false},
			{404, true},
			{499, true},
			{500, false},
		}
		for _, tc := range testCases {
			assert.Equal(tc.expected, cfg.Is(tc.status, ""))
		}
	})

	t.Run("except excludes from exact", func(t *testing.T) {
		cfg := &HTTPStatusConfig{Exact: 400, Except: []int{400}}
		assert.False(cfg.Is(400, ""))
	})

	t.Run("range boundaries", func(t *testing.T) {
		cfg := &HTTPStatusConfig{Range: "400-404"}
		testCases := []struct {
			status   int
			expected bool
		}{
			{400, true},  // lower boundary
			{404, true},  // upper boundary
			{402, true},  // middle
			{399, false}, // below range
			{405, false}, // above range
		}
		for _, tc := range testCases {
			assert.Equal(tc.expected, cfg.Is(tc.status, ""))
		}
	})

	t.Run("both exact and range can match", func(t *testing.T) {
		cfg := &HTTPStatusConfig{Exact: 400, Range: "500-600"}
		testCases := []struct {
			status   int
			expected bool
		}{
			{400, true},  // matches exact
			{500, true},  // matches range
			{550, true},  // matches range
			{450, false}, // matches neither
		}
		for _, tc := range testCases {
			assert.Equal(tc.expected, cfg.Is(tc.status, ""))
		}
	})

	t.Run("no match", func(t *testing.T) {
		cfg := &HTTPStatusConfig{}
		assert.False(cfg.Is(400, ""))
	})

	t.Run("invalid range format - single number", func(t *testing.T) {
		cfg := &HTTPStatusConfig{Range: "400"}
		assert.False(cfg.Is(400, ""))
	})

	t.Run("invalid range format - non-numeric", func(t *testing.T) {
		cfg := &HTTPStatusConfig{Range: "abc-def"}
		assert.False(cfg.Is(400, ""))
	})
}

func TestHTTPStatusConfig_Is_BodyFail(t *testing.T) {
	assert := assert2.New(t)

	t.Run("fail matches", func(t *testing.T) {
		cfg := &HTTPStatusConfig{
			Exact: 400,
			Body: &BodyMatchConfig{
				Fail: []BodyMatchCondition{{Path: "code", Equals: 496}},
			},
		}
		assert.True(cfg.Is(400, `{"code": 496}`))
	})

	t.Run("fail does not match - default fail", func(t *testing.T) {
		cfg := &HTTPStatusConfig{
			Exact: 400,
			Body: &BodyMatchConfig{
				Fail: []BodyMatchCondition{{Path: "code", Equals: 496}},
			},
		}
		// default is "fail", so even though fail condition didn't match, we still fail
		assert.True(cfg.Is(400, `{"code": 500}`))
	})

	t.Run("fail does not match - default except", func(t *testing.T) {
		cfg := &HTTPStatusConfig{
			Exact: 400,
			Body: &BodyMatchConfig{
				Default: "except",
				Fail:    []BodyMatchCondition{{Path: "code", Equals: 496}},
			},
		}
		// default is "except", fail didn't match -> don't fail
		assert.False(cfg.Is(400, `{"code": 500}`))
	})

	t.Run("fail matches but status does not", func(t *testing.T) {
		cfg := &HTTPStatusConfig{
			Exact: 400,
			Body: &BodyMatchConfig{
				Fail: []BodyMatchCondition{{Path: "code", Equals: 496}},
			},
		}
		assert.False(cfg.Is(401, `{"code": 496}`))
	})

	t.Run("dotted path traversal", func(t *testing.T) {
		cfg := &HTTPStatusConfig{
			Exact: 400,
			Body: &BodyMatchConfig{
				Fail: []BodyMatchCondition{{Path: "error.detail.code", Equals: 496}},
			},
		}
		assert.True(cfg.Is(400, `{"error": {"detail": {"code": 496}}}`))
	})

	t.Run("multiple body.fail conditions AND-ed", func(t *testing.T) {
		cfg := &HTTPStatusConfig{
			Exact: 400,
			Body: &BodyMatchConfig{
				Default: "except",
				Fail: []BodyMatchCondition{
					{Path: "code", Equals: 496},
					{Path: "message", Equals: "System malfunction"},
				},
			},
		}
		assert.True(cfg.Is(400, `{"code": 496, "message": "System malfunction"}`))
		assert.False(cfg.Is(400, `{"code": 496, "message": "Other error"}`))
		assert.False(cfg.Is(400, `{"code": 500, "message": "System malfunction"}`))
	})
}

func TestHTTPStatusConfig_Is_BodyExcept(t *testing.T) {
	assert := assert2.New(t)

	t.Run("except matches - don't fail", func(t *testing.T) {
		cfg := &HTTPStatusConfig{
			Exact: 400,
			Body: &BodyMatchConfig{
				Except: []BodyMatchCondition{{Path: "code", Equals: 496}},
			},
		}
		assert.False(cfg.Is(400, `{"code": 496}`))
	})

	t.Run("except does not match - default fail", func(t *testing.T) {
		cfg := &HTTPStatusConfig{
			Exact: 400,
			Body: &BodyMatchConfig{
				Except: []BodyMatchCondition{{Path: "code", Equals: 496}},
			},
		}
		assert.True(cfg.Is(400, `{"code": 500}`))
	})

	t.Run("except does not match - default except", func(t *testing.T) {
		cfg := &HTTPStatusConfig{
			Exact: 400,
			Body: &BodyMatchConfig{
				Default: "except",
				Except:  []BodyMatchCondition{{Path: "code", Equals: 496}},
			},
		}
		// default is "except", except didn't match -> don't fail
		assert.False(cfg.Is(400, `{"code": 500}`))
	})

	t.Run("except with dotted path", func(t *testing.T) {
		cfg := &HTTPStatusConfig{
			Range: "400-499",
			Body: &BodyMatchConfig{
				Except: []BodyMatchCondition{{Path: "error.code", Equals: 496}},
			},
		}
		assert.False(cfg.Is(400, `{"error": {"code": 496}}`))
		assert.True(cfg.Is(400, `{"error": {"code": 500}}`))
	})

	t.Run("except with non-JSON body - default fail", func(t *testing.T) {
		cfg := &HTTPStatusConfig{
			Exact: 400,
			Body: &BodyMatchConfig{
				Except: []BodyMatchCondition{{Path: "code", Equals: 496}},
			},
		}
		assert.True(cfg.Is(400, "not json"))
	})

	t.Run("except with missing path - default fail", func(t *testing.T) {
		cfg := &HTTPStatusConfig{
			Exact: 400,
			Body: &BodyMatchConfig{
				Except: []BodyMatchCondition{{Path: "missing", Equals: 496}},
			},
		}
		assert.True(cfg.Is(400, `{"code": 496}`))
	})

	t.Run("multiple except conditions AND-ed", func(t *testing.T) {
		cfg := &HTTPStatusConfig{
			Exact: 400,
			Body: &BodyMatchConfig{
				Except: []BodyMatchCondition{
					{Path: "code", Equals: 496},
					{Path: "retry", Equals: true},
				},
			},
		}
		// both match - don't fail
		assert.False(cfg.Is(400, `{"code": 496, "retry": true}`))
		// only one matches - default fail
		assert.True(cfg.Is(400, `{"code": 496, "retry": false}`))
		assert.True(cfg.Is(400, `{"code": 500, "retry": true}`))
	})

	t.Run("status doesn't match - never fail regardless of except", func(t *testing.T) {
		cfg := &HTTPStatusConfig{
			Exact: 400,
			Body: &BodyMatchConfig{
				Except: []BodyMatchCondition{{Path: "code", Equals: 496}},
			},
		}
		assert.False(cfg.Is(401, `{"code": 500}`))
	})
}

func TestHTTPStatusConfig_Is_BodyFailAndExcept(t *testing.T) {
	assert := assert2.New(t)

	t.Run("both match - default fail wins", func(t *testing.T) {
		cfg := &HTTPStatusConfig{
			Exact: 400,
			Body: &BodyMatchConfig{
				Fail:   []BodyMatchCondition{{Path: "code", Equals: 496}},
				Except: []BodyMatchCondition{{Path: "code", Equals: 496}},
			},
		}
		assert.True(cfg.Is(400, `{"code": 496}`))
	})

	t.Run("both match - default except wins", func(t *testing.T) {
		cfg := &HTTPStatusConfig{
			Exact: 400,
			Body: &BodyMatchConfig{
				Default: "except",
				Fail:    []BodyMatchCondition{{Path: "code", Equals: 496}},
				Except:  []BodyMatchCondition{{Path: "code", Equals: 496}},
			},
		}
		assert.False(cfg.Is(400, `{"code": 496}`))
	})

	t.Run("fail doesn't match, except matches - don't fail", func(t *testing.T) {
		cfg := &HTTPStatusConfig{
			Exact: 400,
			Body: &BodyMatchConfig{
				Fail:   []BodyMatchCondition{{Path: "code", Equals: 496}},
				Except: []BodyMatchCondition{{Path: "code", Equals: 500}},
			},
		}
		assert.False(cfg.Is(400, `{"code": 500}`))
	})

	t.Run("neither matches - default fail", func(t *testing.T) {
		cfg := &HTTPStatusConfig{
			Exact: 400,
			Body: &BodyMatchConfig{
				Fail:   []BodyMatchCondition{{Path: "code", Equals: 496}},
				Except: []BodyMatchCondition{{Path: "code", Equals: 500}},
			},
		}
		assert.True(cfg.Is(400, `{"code": 999}`))
	})

	t.Run("neither matches - default except", func(t *testing.T) {
		cfg := &HTTPStatusConfig{
			Exact: 400,
			Body: &BodyMatchConfig{
				Default: "except",
				Fail:    []BodyMatchCondition{{Path: "code", Equals: 496}},
				Except:  []BodyMatchCondition{{Path: "code", Equals: 500}},
			},
		}
		assert.False(cfg.Is(400, `{"code": 999}`))
	})
}

func TestHTTPStatusConfig_Is_BackwardCompat(t *testing.T) {
	assert := assert2.New(t)

	t.Run("no body config - always fail on status match", func(t *testing.T) {
		cfg := &HTTPStatusConfig{Exact: 400}
		assert.True(cfg.Is(400, `{"code": 496}`))
		assert.True(cfg.Is(400, ""))
		assert.True(cfg.Is(400, "not json"))
	})
}

func TestHTTPStatusMatchConfig_Is(t *testing.T) {
	assert := assert2.New(t)

	t.Run("single", func(t *testing.T) {
		cfg := HTTPStatusMatchConfig{
			{Exact: 400},
		}
		assert.True(cfg.Is(400, ""))
		assert.False(cfg.Is(401, ""))
	})

	t.Run("range", func(t *testing.T) {
		cfg := HTTPStatusMatchConfig{
			{Range: "400-404"},
		}
		assert.True(cfg.Is(400, ""))
		assert.True(cfg.Is(404, ""))
		assert.False(cfg.Is(405, ""))
	})

	t.Run("multiple", func(t *testing.T) {
		cfg := HTTPStatusMatchConfig{
			{Exact: 400},
			{Range: "500-600"},
		}
		assert.True(cfg.Is(400, ""))
		assert.True(cfg.Is(500, ""))
		assert.True(cfg.Is(501, ""))
		assert.True(cfg.Is(600, ""))
		assert.False(cfg.Is(401, ""))
	})

	t.Run("empty config", func(t *testing.T) {
		cfg := HTTPStatusMatchConfig{}
		assert.False(cfg.Is(400, ""))
		assert.False(cfg.Is(500, ""))
	})

	t.Run("overlapping ranges", func(t *testing.T) {
		cfg := HTTPStatusMatchConfig{
			{Range: "400-450"},
			{Range: "440-500"},
		}
		assert.True(cfg.Is(400, ""))
		assert.True(cfg.Is(445, "")) // in both ranges
		assert.True(cfg.Is(500, ""))
		assert.False(cfg.Is(350, ""))
		assert.False(cfg.Is(550, ""))
	})

	t.Run("body.fail with match config", func(t *testing.T) {
		cfg := HTTPStatusMatchConfig{
			{Exact: 400, Body: &BodyMatchConfig{
				Default: "except",
				Fail:    []BodyMatchCondition{{Path: "code", Equals: 496}},
			}},
			{Range: "500-600"},
		}
		assert.True(cfg.Is(400, `{"code": 496}`))
		assert.False(cfg.Is(400, `{"code": 500}`))
		assert.True(cfg.Is(500, ""))
	})

	t.Run("body.except with match config", func(t *testing.T) {
		cfg := HTTPStatusMatchConfig{
			{Range: "400-499", Body: &BodyMatchConfig{
				Except: []BodyMatchCondition{{Path: "code", Equals: 496}},
			}},
		}
		assert.True(cfg.Is(400, `{"code": 500}`))
		assert.False(cfg.Is(400, `{"code": 496}`))
	})
}
