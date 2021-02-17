package jsonextract

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestReader(t *testing.T) {
	tests := []struct {
		arg  string
		want []json.RawMessage
	}{
		{
			`{{ "test": "a" } {}text[] in {}between{}`,
			[]json.RawMessage{
				[]byte(`{ "test": "a" }`),
				[]byte(`{}`),
				[]byte(`[]`),
				[]byte(`{}`),
				[]byte(`{}`),
			},
		},
		{
			`{{{{{ "test": "a" }} }}}}}}{ {}text[] in {}between{}`,
			[]json.RawMessage{
				[]byte(`{ "test": "a" }`),
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
			[]json.RawMessage{[]byte(`{"a": "b"}`)},
		},
		{
			"[1, 3, 55]",
			[]json.RawMessage{[]byte("[1, 3, 55]")},
		},
		{
			"[1, 3, 55, ]",
			nil,
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
			[]json.RawMessage{[]byte(`{
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
}`)},
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
				json.RawMessage([]byte(`{
				"test": "this is a very }{} mean string"	
			}`)),
			},
		},
		{
			`{
				"test": "this is another very ][] mean string"	
			}`,
			[]json.RawMessage{
				[]byte(
					`{
				"test": "this is another very ][] mean string"	
			}`),
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
	}
	for _, tt := range tests {
		t.Run(t.Name(), func(t *testing.T) {
			if gotExtracted, _ := ReaderObjects(strings.NewReader(tt.arg)); !reflect.DeepEqual(gotExtracted, tt.want) {
				t.Errorf("String() = %v, want %v", convert(gotExtracted), convert(tt.want))
			}
		})
	}
}

func convert(m []json.RawMessage) (msgs []string) {
	for _, v := range m {
		msgs = append(msgs, string(v))
	}
	return
}
