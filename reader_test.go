package jsonextract

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"
	"testing/iotest"
)

func TestReader(t *testing.T) {
	for _, tt := range testData {
		t.Run(t.Name(), func(t *testing.T) {
			if gotExtracted, _ := ReaderObjects(strings.NewReader(tt.arg)); !reflect.DeepEqual(gotExtracted, tt.want) {
				t.Errorf("ReaderObjects() = %v, want %v", convert(gotExtracted), convert(tt.want))
			}
		})
	}
}

func TestCallback(t *testing.T) {
	// Test all from testData
	for _, tt := range testData {
		t.Run(t.Name(), func(t *testing.T) {
			var calls int

			err := Reader(strings.NewReader(tt.arg), func(b []byte) error {
				if !bytes.Equal(b, tt.want[calls]) {
					t.Errorf("Reader() callback %d = %s, want %s", calls, string(b), string(tt.want[calls]))
				}

				calls++

				return nil
			})
			if err != nil {
				panic(err)
			}
		})
	}

	t.Run("callback returns error", func(t *testing.T) {
		var testErr = errors.New("test test")

		err := Reader(strings.NewReader(`{}`), func(b []byte) error {
			return testErr
		})
		if err != testErr {
			t.Errorf("Reader() doesn't return error returned by callback")
		}
	})

	t.Run("stop callback using ErrStop", func(t *testing.T) {
		var calls int
		err := Reader(strings.NewReader(`{}{}{}{}{}`), func(b []byte) error {
			calls++
			if calls == 2 {
				return ErrStop
			}
			return nil
		})
		if err != nil {
			t.Errorf("Reader() doesn't return nil when explicitly stopped")
		}
		if calls != 2 {
			t.Errorf("Reader() calls callback %d times instead of the expected 2 times", calls)
		}
	})
}

func TestReaderErr(t *testing.T) {
	var err = fmt.Errorf("test error")

	var testReader io.Reader = iotest.ErrReader(err)

	o, rerr := ReaderObjects(testReader)
	if err != rerr {
		t.Errorf("expected ReaderObjects() to return first read error")
	}
	if len(o) > 0 {
		t.Error("expected ReaderObjects() to return no result on error")
	}
}

func TestExpectations(t *testing.T) {
	// This is an assumption needed so this package works correctly
	// Since this is true, the value passed to callback will always have a length > 0
	t.Run("empty string to be invalid json", func(t *testing.T) {
		if json.Valid([]byte("")) {
			t.Fail()
		}
	})
}

func convert(m []json.RawMessage) (msgs []string) {
	for _, v := range m {
		msgs = append(msgs, string(v))
	}
	return
}

var testData = []struct {
	arg  string
	want []json.RawMessage
}{
	{
		"{			a: 'null',	b: `true`, c: \"false\"		 }",
		[]json.RawMessage{
			[]byte(`{"a":"null","b":"true","c":"false"}`),
		},
	},
	{
		`{{ "test": "a" } {}text[] in {}between{}`,
		[]json.RawMessage{
			[]byte(`{"test":"a"}`),
			[]byte(`{}`),
			[]byte(`[]`),
			[]byte(`{}`),
			[]byte(`{}`),
		},
	},
	{
		`{{{{{ "test": "a" }} }}}}}}{ {}text[] in {}between{}`,
		[]json.RawMessage{
			[]byte(`{"test":"a"}`),
			[]byte(`{}`),
			[]byte(`[]`),
			[]byte(`{}`),
			[]byte(`{}`),
		},
	},

	{
		`{}some {}text[] in {}between{}`,
		[]json.RawMessage{
			[]byte(`{}`),
			[]byte(`{}`),
			[]byte(`[]`),
			[]byte(`{}`),
			[]byte(`{}`),
		},
	},
	{
		`{}{}[]{}{}`,
		[]json.RawMessage{
			[]byte(`{}`),
			[]byte(`{}`),
			[]byte(`[]`),
			[]byte(`{}`),
			[]byte(`{}`),
		},
	},
	{
		`{"a": "b"}`,
		[]json.RawMessage{[]byte(`{"a":"b"}`)},
	},
	{
		"[1, 3, 55]",
		[]json.RawMessage{[]byte("[1,3,55]")},
	},
	{
		"[1, 3, 55, ]",
		[]json.RawMessage{
			[]byte(`[1,3,55]`),
		},
	},
	{
		`{
			"a": "b",
			"c": "trailing comma",
    		}`,
		[]json.RawMessage{
			[]byte(`{"a":"b","c":"trailing comma"}`),
		},
	},
	{
		`{
  "login": "xarantolus",
  "id": 0,
  "node_id": "----",
  "avatar_url": "https://avatars.githubusercontent.com/u/----",
  "gravatar_id": "",
  "url": "https://api.github.com/users/xarantolus",
  "html_url": "https://github.com/xarantolus",
  "followers_url": "https://api.github.com/users/xarantolus/followers",
  "following_url": "https://api.github.com/users/xarantolus/following{/other_user}",
  "gists_url": "https://api.github.com/users/xarantolus/gists{/gist_id}",
  "starred_url": "https://api.github.com/users/xarantolus/starred{/owner}{/repo}",
  "subscriptions_url": "https://api.github.com/users/xarantolus/subscriptions",
  "organizations_url": "https://api.github.com/users/xarantolus/orgs",
  "repos_url": "https://api.github.com/users/xarantolus/repos",
  "events_url": "https://api.github.com/users/xarantolus/events{/privacy}",
  "received_events_url": "https://api.github.com/users/xarantolus/received_events",
  "type": "User",
  "site_admin": false,
  "name": "----",
  "company": null,
  "blog": "----",
  "location": "----",
  "email": "----",
  "hireable": "----",
  "bio": "----",
  "twitter_username": null,
  "public_repos": 17,
  "public_gists": 3,
  "followers": 13,
  "following": 242,
  "created_at": "2017-10-02T18:47:02Z",
  "updated_at": "2021-01-08T20:42:33Z"
}`,
		[]json.RawMessage{[]byte(`{"login":"xarantolus","id":0,"node_id":"----","avatar_url":"https://avatars.githubusercontent.com/u/----","gravatar_id":"","url":"https://api.github.com/users/xarantolus","html_url":"https://github.com/xarantolus","followers_url":"https://api.github.com/users/xarantolus/followers","following_url":"https://api.github.com/users/xarantolus/following{/other_user}","gists_url":"https://api.github.com/users/xarantolus/gists{/gist_id}","starred_url":"https://api.github.com/users/xarantolus/starred{/owner}{/repo}","subscriptions_url":"https://api.github.com/users/xarantolus/subscriptions","organizations_url":"https://api.github.com/users/xarantolus/orgs","repos_url":"https://api.github.com/users/xarantolus/repos","events_url":"https://api.github.com/users/xarantolus/events{/privacy}","received_events_url":"https://api.github.com/users/xarantolus/received_events","type":"User","site_admin":false,"name":"----","company":null,"blog":"----","location":"----","email":"----","hireable":"----","bio":"----","twitter_username":null,"public_repos":17,"public_gists":3,"followers":13,"following":242,"created_at":"2017-10-02T18:47:02Z","updated_at":"2021-01-08T20:42:33Z"}`)},
	},
	{
		"askdflaksmvalsd",
		nil,
	},
	{
		`"json encoded text\nNew line"`,
		nil,
	},
	{
		`{
				"test": "this is a very }{} mean string"	
			}`,
		[]json.RawMessage{
			json.RawMessage([]byte(`{"test":"this is a very }{} mean string"}`)),
		},
	},
	{
		`{
				"test": "this is another very ][] mean string"	
			}`,
		[]json.RawMessage{
			[]byte(
				`{"test":"this is another very ][] mean string"}`),
		},
	},
	{
		`{}some {}text[] in {}between{}`,
		[]json.RawMessage{
			[]byte(`{}`),
			[]byte(`{}`),
			[]byte(`[]`),
			[]byte(`{}`),
			[]byte(`{}`),
		},
	},
	{
		`<script>
    loadScript('/static/js/sidenav.js', {type: 'module', async: true, defer: true})
  </script>`,
		[]json.RawMessage{
			[]byte(`{"type":"module","async":true,"defer":true}`),
		},
	},
	{
		`{'test': "Test"}`,
		[]json.RawMessage{
			[]byte(`{"test":"Test"}`),
		},
	},
	{
		`{
			"a": null,
			"b": true,
			"c": false
		 }`,
		[]json.RawMessage{
			[]byte(`{"a":null,"b":true,"c":false}`),
		},
	},
	{
		`["one", 'two', "three", ]`,
		[]json.RawMessage{
			[]byte(`["one","two","three"]`),
		},
	},
	{
		`{
	// Keys without quotes are valid in JavaScript, but not in JSON
	key: "value",
	num: 295.2,

	// Comments are removed while processing

	// Mixing normal and quotes keys is possible 
	"obj": {
		"quoted": 325,
		unquoted: 'test', // This trailing comma will be removed
	}
}`,
		[]json.RawMessage{
			[]byte(`{"key":"value","num":295.2,"obj":{"quoted":325,"unquoted":"test"}}`),
		},
	},
	{
		`<script>var arr = ["one", 'two &amp; three', "four", ];</script>`,
		[]json.RawMessage{
			[]byte(`["one","two &amp; three","four"]`),
		},
	},
}
