package jsonextract

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
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

			if calls != len(tt.want) {
				t.Errorf("Callback was called %d times, but wanted %d calls", calls, len(tt.want))
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

type failableReader struct {
	r io.Reader

	failNext bool
}

func (f *failableReader) Read(p []byte) (n int, err error) {
	if f.failNext {
		return 0, fmt.Errorf("failed")
	}

	return f.r.Read(p)
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

	r := iotest.OneByteReader(strings.NewReader(strings.Repeat("{}", 2500)))

	o, rerr = ReaderObjects(r)

	if rerr != nil {
		t.Error("Expected ReaderObjects() to return no error")
	}
	if len(o) != 2500 {
		t.Error("ReaderObjects() didn't read the entire reader")
	}

	var cbCount int
	fr := &failableReader{
		r: strings.NewReader("{}{}"),
	}
	rerr = Reader(iotest.OneByteReader(fr), func(b []byte) error {
		cbCount++

		if cbCount == 1 {
			fr.failNext = true
		}

		return nil
	})

	if rerr == nil || cbCount != 1 {
		t.Errorf("Expected Reader to return error after exactly one callback")
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

func TestHTMLFile(t *testing.T) {

	var expectedValues = [][]byte{
		[]byte(`{"value":25,"another":"test","quoted":{"is this even valid in JS?":75},"nextkey":"this\ntemplate literal\n\nspans\n\nmany \n\n\nlines"}`),
		[]byte(`{"subkey":"value"}`),
		[]byte(`{"subkey":"value"}`),
		[]byte(`{"@context":"https://schema.org","@type":"Product","aggregateRating":{"@type":"AggregateRating","ratingValue":"3.5","reviewCount":"11"},"description":"jsonextract is a Go library","name":"jsonextract","image":"microwave.jpg","offers":{"@type":"Offer","availability":"https://schema.org/InStock","price":"00.00","priceCurrency":"USD"},"review":[{"@type":"Review","author":"Ellie","datePublished":"2012-09-06","reviewBody":"I'm still not sure if this works.","name":"Test","reviewRating":{"@type":"Rating","bestRating":"5","ratingValue":"1","worstRating":"1"}},{"@type":"Review","author":"Lucas","datePublished":"2014-02-21","reviewBody":"Great microwave for the price.","name":"Value purchase","reviewRating":{"@type":"Rating","bestRating":"5","ratingValue":"4","worstRating":"1"}}]}`),
		[]byte(`{}`),
		[]byte(`[]`),
		[]byte("[\" this is a template string. \",\"in JS you can escape` the quote character `\"]"),
	}

	f, err := os.Open("testdata/test.html")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	var calls int
	err = Reader(f, func(b []byte) error {
		if !bytes.Equal(expectedValues[calls], b) {
			t.Errorf("Expected value %s to be %s", string(b), string(expectedValues[calls]))
		}

		calls++

		return nil
	})
	if err != nil {
		panic(err)
	}

	if len(expectedValues) != calls {
		t.Errorf("Expected callback to be called %d times, but was only called %d", len(expectedValues), calls)
	}
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
		`
		[]
    <script>
        StackExchange.ready(function () {

                    var graphData = [984,984,1019,1019,1029,1029,1029,1029,1029,1042,1042,1042,1042,1042,1042,1042,1042,1042,1042,1042,1042,1042,1042,1042,1042,1042,1042,1044,1044,1044,1044,1044,1044,1044,1044,1044,1044,1044,1044,1044,1069,1069,1069,1069,1069,1069,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1134,1134,1134,1134,1134,1134,1144,1144,1144,1144,1144,1144,1144,1144,1154,1154,1154,1154,1154,1154,1154,1154,1164,1164,1189,1189,1199,1199,1199,1199,1199,1199,1199,1199,1199,1199,1199,1199,1199,1199,1214,1214,1214,1214,1214,1214,1229,1229,1229,1229,1229,1229,1229,1229,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1264,1264,1264,1289,1289,1289,1289,1289,1289,1289,1289,1289,1289,1289,1289,1289,1289,1289,1289,1289,1314,1314,1324,1324,1324,1324,1334,1334,1334,1334,1334,1334,1334,1334,1334,1334,1334,1334,1334,1334,1334,1334,1334,1334,1334,1334,1334,1334,1334,1334,1334,1334,1334,1334,1334,1334,1334,1334,1334,1344,1344,1344,1344,1344,1354,1354,1354,1354,1354,1354,1354,1354,1354,1354,1354,1354,1354,1354,1354,1354,1354,1354,1354,1354,1354,1354,1354,1354,1354,1354,1354,1354,1354,1354,1354,1354,1354,1354,1354,1354,1354,1354,1354,1354,1354,1354,1364,1364,1364,1364,1364,1364,1364,1364,1364,1364,1364,1364,1374,1374,1374,1384,1384,1394,1394,1394,1394,1394,1394,1394,1394,1394,1394,1394,1394,1394,1394,1394,1394,1394,1394,1394,1404,1404,1404,1404,1404,1404,1404,1404,1404,1404,1414,1414,1414,1414,1424,1424,1424,1424,1424,1424];
                    StackExchange.user.renderMiniGraph(graphData);
                

                    StackExchange.user.userCardMessages.nextPrivInfo = [
                        '<h4 class="popup-title">Create new tags</h4>',
                        '<div class="popup-white">',
                            '<p>Add new tags to the site</p>',
                            '<div class="actions">',
                                '<span class="rep-number">1,424/1,500 Rep.</span>',
                                '<a href="/help/privileges/create-tags" class="s-btn s-btn__primary" title="Learn more">Learn more</a>',
                            '</div>',
                        '</div>'
                    ].join('');
                        });
    </script>`,
		[]json.RawMessage{
			[]byte(`[]`),
			[]byte(`[984,984,1019,1019,1029,1029,1029,1029,1029,1042,1042,1042,1042,1042,1042,1042,1042,1042,1042,1042,1042,1042,1042,1042,1042,1042,1042,1044,1044,1044,1044,1044,1044,1044,1044,1044,1044,1044,1044,1044,1069,1069,1069,1069,1069,1069,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1124,1134,1134,1134,1134,1134,1134,1144,1144,1144,1144,1144,1144,1144,1144,1154,1154,1154,1154,1154,1154,1154,1154,1164,1164,1189,1189,1199,1199,1199,1199,1199,1199,1199,1199,1199,1199,1199,1199,1199,1199,1214,1214,1214,1214,1214,1214,1229,1229,1229,1229,1229,1229,1229,1229,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1239,1264,1264,1264,1289,1289,1289,1289,1289,1289,1289,1289,1289,1289,1289,1289,1289,1289,1289,1289,1289,1314,1314,1324,1324,1324,1324,1334,1334,1334,1334,1334,1334,1334,1334,1334,1334,1334,1334,1334,1334,1334,1334,1334,1334,1334,1334,1334,1334,1334,1334,1334,1334,1334,1334,1334,1334,1334,1334,1334,1344,1344,1344,1344,1344,1354,1354,1354,1354,1354,1354,1354,1354,1354,1354,1354,1354,1354,1354,1354,1354,1354,1354,1354,1354,1354,1354,1354,1354,1354,1354,1354,1354,1354,1354,1354,1354,1354,1354,1354,1354,1354,1354,1354,1354,1354,1354,1364,1364,1364,1364,1364,1364,1364,1364,1364,1364,1364,1364,1374,1374,1374,1384,1384,1394,1394,1394,1394,1394,1394,1394,1394,1394,1394,1394,1394,1394,1394,1394,1394,1394,1394,1394,1404,1404,1404,1404,1404,1404,1404,1404,1404,1404,1414,1414,1414,1414,1424,1424,1424,1424,1424,1424]`),
			[]byte(`["<h4 class=\"popup-title\">Create new tags</h4>","<div class=\"popup-white\">","<p>Add new tags to the site</p>","<div class=\"actions\">","<span class=\"rep-number\">1,424/1,500 Rep.</span>","<a href=\"/help/privileges/create-tags\" class=\"s-btn s-btn__primary\" title=\"Learn more\">Learn more</a>","</div>","</div>"]`),
		},
	},
	{`["<h4 class=\"popup-title\">Create new tags</h4>","<div class=\"popup-white\">","<p>Add new tags to the site</p>","<div class=\"actions\">","<span class=\"rep-number\">1,424/1,500 Rep.</span>","<a href=\"/help/privileges/create-tags\" class=\"s-btn s-btn__primary\" title=\"Learn more\">Learn more</a>","</div>","</div>"]`,

		[]json.RawMessage{
			[]byte(`["<h4 class=\"popup-title\">Create new tags</h4>","<div class=\"popup-white\">","<p>Add new tags to the site</p>","<div class=\"actions\">","<span class=\"rep-number\">1,424/1,500 Rep.</span>","<a href=\"/help/privileges/create-tags\" class=\"s-btn s-btn__primary\" title=\"Learn more\">Learn more</a>","</div>","</div>"]`),
		},
	},
	{
		`StackExchange.user.userCardMessages.nextPrivInfo = [
                        '<h4 class="popup-title">Create new tags</h4>',
                        '<div class="popup-white">',
                            '<p>Add new tags to the site</p>',
                            '<div class="actions">',
                                '<span class="rep-number">1,424/1,500 Rep.</span>',
                                '<a href="/help/privileges/create-tags" class="s-btn s-btn__primary" title="Learn more">Learn more</a>',
                            '</div>',
                        '</div>'
                    ].join('');`,
		[]json.RawMessage{
			[]byte(`["<h4 class=\"popup-title\">Create new tags</h4>","<div class=\"popup-white\">","<p>Add new tags to the site</p>","<div class=\"actions\">","<span class=\"rep-number\">1,424/1,500 Rep.</span>","<a href=\"/help/privileges/create-tags\" class=\"s-btn s-btn__primary\" title=\"Learn more\">Learn more</a>","</div>","</div>"]`),
		},
	},
	{
		`[15, 17, -3]`,
		[]json.RawMessage{
			[]byte(`[15,17,-3]`),
		},
	},
	{
		// In JS, we can escape a ` in a template literal
		"{ key: ` \\` ` }",
		[]json.RawMessage{
			[]byte("{\"key\":\" ` \"}"),
		},
	},
	{
		"[`Template quotes`]",
		[]json.RawMessage{
			[]byte("[\"Template quotes\"]"),
		},
	},
	{
		// The \n gets escaped by Go
		"{ 'key': `this is a\nmultline JavaScript string` }",
		[]json.RawMessage{
			[]byte(`{"key":"this is a\nmultline JavaScript string"}`),
		},
	},
	{
		"[`Template quotes inside of template quotes can be escaped using \\``]",
		[]json.RawMessage{
			[]byte("[\"Template quotes inside of template quotes can be escaped using `\"]"),
		},
	},
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
	{
		// Regex fields should be escaped as a normal string,
		// no need to throw away useful data that we might want to extract
		`{"key":  /test/i, useful_data: { "a": "b" }, another_value_we_might_want:"c" }`,
		[]json.RawMessage{
			[]byte(`{"key":"/test/i","useful_data":{"a":"b"},"another_value_we_might_want":"c"}`),
		},
	},
	{
		`{"num": 3+3 }`,
		nil,
	},
	{
		`{expr: null || "fallback string" }`,
		nil,
	},
	{
		strings.Repeat("{", 250) + strings.Repeat("}", 100),
		[]json.RawMessage{
			[]byte("{}"),
		},
	},
	{
		strings.Repeat("[", 100) + "]",
		[]json.RawMessage{
			[]byte("[]"),
		},
	},
	{
		"[\"" + strings.Repeat("long string ", 100) + "]",
		nil,
	},
}

type infiniteReader struct {
	initial *strings.Reader

	rest []byte

	reads int
}

func (i *infiniteReader) Read(p []byte) (n int, err error) {
	n, err = i.initial.Read(p)

	for {
		i.reads++
		if n >= len(p) {
			return len(p), nil
		}

		// Almost infinite?
		if i.reads > 10_000 {
			panic("infiniteReader has read too many times")
		}

		n += copy(p[n:], i.rest)
	}
}

var readerTestData = []struct {
	input string
	want  string
}{
	{
		"{this is included} but not this",
		"{this is included}",
	},
	{
		`{
			"a rather": "valid json object",
			"it even": {
				"has": [
					"arrays",
					"in",
					"it",
				]
			}	
		} but what happened if we cut this off?`,
		`{
			"a rather": "valid json object",
			"it even": {
				"has": [
					"arrays",
					"in",
					"it",
				]
			}	
		}`,
	},
	{
		"[` Including escaped backticks shouldn't be a problem \\``]",
		"[` Including escaped backticks shouldn't be a problem \\``]",
	},
	{
		`{"just like \"": "any other 'quotes' " } hmm`,
		`{"just like \"": "any other 'quotes' " }`,
	},
	{
		`{{{{{{{}}}}}}}}}`,
		`{{{{{{{}}}}}}}`,
	},
	{
		`[[[[[[[[[[[[[[[[["ye\"et"]]]]]]]]]]]]]]]]]]]]]]]]]]`,
		`[[[[[[[[[[[[[[[[["ye\"et"]]]]]]]]]]]]]]]]]`,
	},
	{
		`{ ` + strings.Repeat("a", 100) + "}",
		`{ ` + strings.Repeat("a", 100) + "}",
	},
	{
		"['ayy \\'', \"lmao\\\"\"]",
		`['ayy \'', "lmao\""]`,
	},
	{
		"[` 'quotes' inside of \"other quotes\"`, 'but wait, there are `more`']]]]]]]]]]]]]]}]]",
		"[` 'quotes' inside of \"other quotes\"`, 'but wait, there are `more`']",
	},
	{
		"[1,2,3,4,5,6,7,8,9,10];",
		"[1,2,3,4,5,6,7,8,9,10]",
	},
}

func TestResettableRuneBuffer(t *testing.T) {
	for _, tt := range readerTestData {
		t.Run(t.Name(), func(t *testing.T) {
			var r = &resettableRuneBuffer{
				normalBuffer: bufio.NewReader(strings.NewReader(tt.input)),
			}

			r.MarkStart()

			err := iotest.TestReader(r, []byte(tt.input))
			if err != nil {
				t.Errorf("Invalid resettableRuneBuffer implementation (initial read): %s", err.Error())
			}

			err = r.ReturnAndSkip(len(tt.input) / 2)
			if err != nil {
				panic(err)
			}

			r.MarkEnd()

			err = iotest.TestReader(r, []byte(tt.input[len(tt.input)/2:]))
			if err != nil {
				t.Errorf("Invalid resettableRuneBuffer implementation (after returning): %s", err.Error())
			}
		})
	}
}

// Test to check if the example program still works
func TestStackOverflow(t *testing.T) {
	// Running in GitHub actions? Skip this
	if os.Getenv("CI") == "true" {
		t.Skip()
	}

	resp, err := http.Get("https://stackoverflow.com/users/5728357/xarantolus?tab=topactivity")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	var yValues []float64

	err = Reader(resp.Body, func(b []byte) error {
		if b[0] == '{' {
			return nil
		}

		// Try to unmarshal
		err := json.Unmarshal(b, &yValues)
		if err == nil && len(yValues) > 10 {
			// If it was successful, we stop parsing
			return ErrStop
		}

		// continue with next object
		return nil
	})
	if err != nil {
		panic("cannot extract JSON objects: " + err.Error())
	}

	if len(yValues) <= 10 {
		t.Errorf("Couldn't find JSON data: len(yValues) = %d", len(yValues))
	}
}
