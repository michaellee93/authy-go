package authy

//Package for interacting with authy API for 2FA

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"
	"github.com/google/go-querystring/query"
	"log"
)

// Example usage
// var app = authy.App{
// 	ApiSecret: os.GetEnv("AUTHY_API_SECRET"),
// }
// func main() {
// 	var authyUserID int64 = 34015850845
// 	client := authy.NewClient(app)
// 	msg, err := client.SendOTP(authyUserID)
// 	fmt.Printf("%+v", msg)
// 	if err != nil {
// 		print(err)
// 	}
// 	token := ""
// 	fmt.Println("ENTER TOKEN")
// 	scanner := bufio.NewScanner(os.Stdin)
// 	scanner.Scan()
// 	token = scanner.Text()
// 	success, err := client.CheckOTPToken(authyUserID, token)
// 	if success {
// 		println("Successfully authenticated user")
// 	} else {
// 		println(err)
// 	}
// }

//alter table users add column authy_id integer not null default 0;
//alter table users add column authy_enabled bool not null default false;

var baseUrl = "https://api.authy.com/protected/"

// Client for interacting with the Authy API
type Client struct {
	Client  *http.Client
	app     App
	baseURL *url.URL
}

type App struct {
	ApiSecret string
	ApiFormat string //xml or json defaults to json if not provided
}

// NewClient returns a client to make requests to the Authy API
func NewClient(a App) *Client {
	urlWithFormat := baseUrl + "json/"
	if a.ApiFormat == "xml" {
		urlWithFormat = baseUrl + "xml/"
	}

	url, err := url.Parse(urlWithFormat)
	if err != nil {
		return nil
	}

	return &Client{
		Client:  &http.Client{Timeout: time.Second * 20},
		app:     a,
		baseURL: url,
	}
}

// NewRequest creates a new request with the given method, path and marshals the given
// body into url encoded data
func (c *Client) NewRequest(method, relPath string, body interface{}) (*http.Request, error) {
	rel, err := url.Parse(relPath)
	if err != nil {
		return nil, err
	}

	// Make the full url based on the relative path
	u := c.baseURL.ResolveReference(rel)

	var out url.Values
	if body != nil {
		out, err = query.Values(body)
		if err != nil {
			return nil, err
		}
	}

	req, err := http.NewRequest(method, u.String(), strings.NewReader(out.Encode()))
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Accept", "application/json")
	req.Header.Add("User-Agent", "authy-go-client")
	req.Header.Add("X-Authy-API-Key", c.app.ApiSecret)
	return req, nil
}

// GetAppInfo gets the app info for the provided API secret
func (c *Client) GetAppInfo() (*ResponseMessage, error) {
	info := new(ResponseMessage)
	c.Get("app/details", info)
	return info, nil
}

// Get takes a relative path to which it makes a GET request and returns
// reads the response data into the resource provided
func (c *Client) Get(relPath string, resource interface{}) error {
	req, err := c.NewRequest("GET", relPath, nil)
	if err != nil {
		return err
	}

	resp, err := c.Client.Do(req)
	if err != nil {
		return err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	json.Unmarshal(body, resource)
	return nil
}

// Post to Authy API based on path provided
func (c *Client) Post(relPath string, body interface{}, resource interface{}) error {
	req, err := c.NewRequest("POST", relPath, body)
	if err != nil {
		return err
	}

	resp, err := c.Client.Do(req)
	if err != nil {
		return err
	}

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	json.Unmarshal(respBody, resource)
	return nil
}

// the app data returned from the app endpoint
type authyAppInfo struct {
	Name              string `json:"name"`
	Plan              string `json:"plan"`
	SmsEnabled        bool   `json:"sms_enabled"`
	PhoneCallsEnabled bool   `json:"phone_calls_enabled"`
	AppID             int64  `json:"app_id"`
	OnetouchEnabled   bool   `json:"onetouch_enabled"`
}

// ResponseMessage is the wrapper for the data returned by the authy API
type ResponseMessage struct {
	App     authyAppInfo `json:"app"`
	User    user         `json:"user"`
	Status  status       `json:"status"`
	Device  device       `json:"device"`
	Token   string       `json:"token"`
	Message string       `json:"message"`
	Success bool         `json:"success"`
}

// embedded user data in API response from user status enpoint
type user struct {
	ID int64 `json:"id"`
}

// AuthyUser is for use when creating users with Authy API
// the new user endpoitn expects at lease the cellphone and country code params
type AuthyUser struct {
	Email           string `url:"user[email],omitempty"`
	Cellphone       string `url:"user[cellphone]"`
	CountryCode     string `url:"user[country_code]"`
	SendInstallLink bool   `url:"send_install_link_via_sms,omitempty"`
}

// CreateUser creates a user - must provide cellphone number
// and country code for request to be processed
func (c *Client) CreateUser(au AuthyUser) (int64, error) {
	if au.Cellphone == "" || au.CountryCode == "" {
		return 0, fmt.Errorf("AUTHY: insufficient data provided to create user")
	}

	resource := new(ResponseMessage)
	err := c.Post("users/new", au, resource)
	if err != nil {
		return 0, err
	}

	if !resource.Success {
		return 0, fmt.Errorf("AUTHY: create not successful %v", resource.Message)
	}

	return resource.User.ID, nil
}

// RemoveUser removes a user from Authy API
func (c *Client) RemoveUser(authyUserID int64) error {
	path := fmt.Sprintf("users/%d/remove", authyUserID)
	resource := new(ResponseMessage)
	err := c.Post(path, nil, resource)
	if err != nil {
		return err
	}

	if !resource.Success {
		return fmt.Errorf("%v", resource.Message)
	}

	return nil
}

// UserStatus requests the current status of the provided user ID
// in the authy API
func (c *Client) UserStatus(authyUserID int64) (*ResponseMessage, error) {
	path := fmt.Sprintf("users/%d/status", authyUserID)
	msg := new(ResponseMessage)
	err := c.Get(path, msg)
	if err != nil {
		return nil, err
	}
	return msg, nil
}

type status struct {
	AuthyID     int64  `json:"authy_id"`
	Confirmed   bool   `json:"confirmed"`
	Registered  bool   `json:"registered"`
	CountryCode int    `json:"country_code"`
	PhoneNumber string `json:"phone_number"`
	Email       string `json:"email"`
}

// SendOTP triggers a OTP to be sent to the user based on their authy ID
// requires a user to be already added to authy
func (c *Client) SendOTP(authyUserID int64) (*ResponseMessage, error) {
	return c.SendOTPWithAction(authyUserID, "", "")
}

// SendOTPWithAction triggers a OTP to be sent to the user based with a
// custom message on their authy ID requires a user to be already added to authy
// https://www.twilio.com/docs/authy/api/one-time-passwords
func (c *Client) SendOTPWithAction(authyUserID int64, action, actionMessage string) (*ResponseMessage, error) {
	path := fmt.Sprintf("sms/%d", authyUserID)
	if action != "" {
		//doesn't work?
		path = fmt.Sprintf("%s?action=%s", path, action)
		//doesn't work
		if actionMessage != "" {
			path = fmt.Sprintf("%s&action_message=%s", path, actionMessage)
		}
	}
	msg := new(ResponseMessage)
	err := c.Get(path, msg)
	if err != nil {
		return msg, err
	}
	return msg, nil
}

// CheckOTPToken checks with authy API whether the provided token is
// valid in order to grant access - response
// can't use standard response message with this endpoint because it returns "true" rather than true
// for json values - could write a customer UnmarshalJSON for the struct to clean it up
// this method is really ugly because the authy API sends back different types for true (string) and false (bool)
// it currently throws an error on unmarshal instead of denying based on the reading of the response
func (c *Client) CheckOTPToken(authyUserID int64, token string) (bool, error) {
	if authyUserID == 0 || token == "" {
		return false, fmt.Errorf("authyUserID or token not provided")
	}

	path := fmt.Sprintf("verify/%s/%d", token, authyUserID)
	req, err := c.NewRequest("GET", path, nil)
	if err != nil {
		return false, err
	}

	resp, err := c.Client.Do(req)
	if err != nil {
		return false, err
	}

	if resp.StatusCode != 200 {
		return false, fmt.Errorf("invalid token")
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println("authy-go CheckOTPToken: malformed response")
		return false, err
	}

	msg := struct {
		Success string `json:"success"`
		Token   string `json:"token"`
	}{}

	err = json.Unmarshal(body, &msg)
	if err != nil {
		log.Println("authy-go CheckOTPToken: error unmarshaling authy API response")
		//log.Error().Err(err).Msg("error unmarshaling authy API response")
		return false, err
	}

	if msg.Success == "true" && msg.Token == "is valid" {
		return true, nil
	}
	return false, nil
}

type device struct {
	ID     int64   `json:"id"`
	OSType *string `json:"os_type"`
	/*	RegistrationDate      *string `json:"registration_date"`
		RegistrationMethod    *string `json:"registration_method"`
		RegistrationRegion    *string `json:"registration_region"`
		RegistrationCity      *string `json:"registration_city"`
		Country               *string `json:"country"`
		Region                *string `json:"region"`
		City                  *string `json:"city"`
		IP                    *string `json:"ip"`
		LastAccountRecoveryAt *string `json:"last_account_recovery_at"`
		LastSyncDate          *string `json:"last_sync_date"`*/
}
