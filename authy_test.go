package authy

import (
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/jarcoal/httpmock"
)

var (
	app    App
	client *Client
)

func setup() {
	app = App{
		ApiSecret: "verysecret",
	}
	client = NewClient(app)
	httpmock.ActivateNonDefault(client.Client)
}

func teardown() {
	httpmock.DeactivateAndReset()
}

func TestNewClient(t *testing.T) {
	testClient := NewClient(app)
	expected := "https://api.authy.com/protected/json/"
	if app.ApiFormat != "" {
		expected = "https://api.authy.com/protected/xml/"
	}

	if testClient.baseURL.String() != expected {
		t.Errorf("NewClient BaseURL = %v, expected %v", testClient.baseURL.String(), expected)
	}
}

func TestNewRequest(t *testing.T) {
	setup()
	defer teardown()

	testClient := NewClient(app)

	apiFormat := "json"
	if app.ApiFormat != "" {
		apiFormat = "xml"
	}

	inURL, outURL := "some/thing", fmt.Sprintf("https://api.authy.com/protected/%s/some/thing", apiFormat)
	inBody := struct {
		Hello string `url:"hello"`
	}{Hello: "World"}
	outBody := `hello=World`

	type extraOptions struct {
		Limit int `url:"limit"`
	}

	req, err := testClient.NewRequest("GET", inURL, inBody)
	if err != nil {
		t.Fatalf("NewRequest(%v) err = %v, expected nil", inURL, err)
	}

	// Test relative URL was expanded
	if req.URL.String() != outURL {
		t.Errorf("NewRequest(%v) URL = %v, expected %v", inURL, req.URL, outURL)
	}

	// Test body was URL encoded
	body, _ := ioutil.ReadAll(req.Body)
	if string(body) != outBody {
		t.Errorf("NewRequest(%v) Body = %v, expected %v", inBody, string(body), outBody)
	}

	// Test token is attached to the request
	token := req.Header.Get("X-Authy-API-Key")
	expected := "verysecret"
	if token != expected {
		t.Errorf("NewRequest() X-Authy-API-Key = %v, expected %v", token, expected)
	}
}

func TestCheckOTPToken(t *testing.T) {
	setup()
	defer teardown()

	cases := []struct {
		userID    int64
		token     string
		responder httpmock.Responder
		expected  interface{}
	}{
		{
			1234567,
			"atokenforyou",
			httpmock.NewStringResponder(200,
				`{
							"message": "Token is valid.", 
							"token": "is valid", 
							"success": "true"
							}`),
			true,
		},
		{
			1234567,
			"abadtokenforyou",
			httpmock.NewStringResponder(401, `
				{
					{
						"message": "Token is invalid. Token was used recently",
						"success": false,
					}
				}`),
			false,
		},
		{
			0,
			"",
			httpmock.NewStringResponder(401,
				`{
					{
						"message": "Token is invalid. Token was used recently",
						"success": false,
					}
				}`),
			false,
		},
		{
			0,
			"",
			httpmock.NewStringResponder(200,
				`{
					{
						"message": "Token is invalid. Token was used recently",
						"success": false,
					}
				}`),
			false,
		},
		{
			0,
			"",
			httpmock.NewStringResponder(401, ``),
			false,
		},
	}

	for _, c := range cases {
		url := fmt.Sprintf("https://api.authy.com/protected/json/verify/%s/%d", c.token, c.userID)
		httpmock.RegisterResponder("GET", url, c.responder)

		success, _ := client.CheckOTPToken(c.userID, c.token)
		s, _ := c.expected.(bool)

		if success != s {
			t.Errorf("CheckOTPToken failed got %v when expected %v", success, c.expected)
		}
	}
}

func TestSendOTP(t *testing.T) {
	setup()
	defer teardown()

	cases := []struct {
		userID    int64
		responder httpmock.Responder
		expected  interface{}
	}{
		{
			12334566,
			httpmock.NewStringResponder(200, `{"success": true}`),
			true,
		},
		{
			12334566,
			httpmock.NewStringResponder(401, `{"success": false}`),
			false,
		},
		{
			12334566,
			httpmock.NewStringResponder(401, ``),
			false,
		},
	}

	for _, c := range cases {
		url := fmt.Sprintf("https://api.authy.com/protected/json/sms/%v", c.userID)

		httpmock.RegisterResponder("GET", url, c.responder)

		msg, _ := client.SendOTP(c.userID)
		exp, _ := c.expected.(bool)
		if msg.Success != exp {
			t.Errorf("SendOTP: got %v expected %v", msg.Success, exp)
		}
	}
}

func TestCreateUser(t *testing.T) {
	setup()
	defer teardown()

	cases := []struct {
		user      AuthyUser
		responder httpmock.Responder
		expected  int64
	}{
		{
			AuthyUser{
				Cellphone:   "111111111",
				CountryCode: "61",
			},
			httpmock.NewStringResponder(201, `
						{
						"success":true, 
						"user":{
							"id":12345
							}
						}`),
			12345,
		}, {
			AuthyUser{
				Cellphone:   "",
				CountryCode: "",
			},
			httpmock.NewStringResponder(400, `
			{
				"success":false, 
			}`),
			0,
		}, {
			AuthyUser{
				Cellphone:   "111111111",
				CountryCode: "111",
			},
			httpmock.NewStringResponder(400, `
			{
				"success":false, 
			}`),
			0,
		},
	}

	for _, c := range cases {
		httpmock.RegisterResponder("POST", "https://api.authy.com/protected/json/users/new", c.responder)
		id, err := client.CreateUser(c.user)

		if c.expected == 0 && err == nil {
			t.Errorf("returned 0 value with no error")
		}

		if id != c.expected {
			t.Errorf("CreateUser expected %v got %v", c.expected, id)
		}

	}
}
