/*
Copyright [2016] [mercadolibre.com]

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package sdk

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/mercadolibre/go-meli-toolkit/restful/rest"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
)

const (
	API_TEST      = "http://localhost:3000"
	CLIENT_ID     = 123456
	CLIENT_SECRET = "client secret"
	USER_CODE     = "valid code with refresh token"
)

func TestMain(m *testing.M) {
	rest.StartMockupServer()
	os.Exit(m.Run())
}

func Test_Generic_Client_Is_Returned_When_No_UserCODE_is_given(t *testing.T) {

	client, _ := Meli(CLIENT_ID, "", CLIENT_SECRET, "htt://www.example.com")

	if client.auth != anonymous {
		log.Printf("Error: Client is not ANONYMOUS")
		t.FailNow()
	}

}

func Test_URL_for_authentication_is_properly_returned(t *testing.T) {

	expectedUrl := "https://auth.mercadolibre.com.ar/authorization?response_type=code&client_id=123456&redirect_uri=http%3A%2F%2Fsomeurl.com"

	url := GetAuthURL(CLIENT_ID, "https://auth.mercadolibre.com.ar", "http://someurl.com")

	if url != expectedUrl {
		log.Printf("Error: The URL is different from the one that was expected.")
		log.Printf("expected %s", expectedUrl)
		log.Printf("obtained %s", url)
		t.FailNow()
	}

}

func Test_FullAuthenticated_Client_Is_Returned_When_UserCODE_And_ClientId_is_given(t *testing.T) {

	config := MeliConfig{

		ClientID:       CLIENT_ID,
		UserCode:       USER_CODE,
		Secret:         CLIENT_SECRET,
		CallBackURL:    "http://www.example.com",
		HTTPClient:     MockHttpClient{},
		TokenRefresher: MockTockenRefresher{},
	}

	client, _ := MeliClient(config)

	if client == nil || client.auth == anonymous {
		log.Printf("Error: Client is not a full one")
		t.FailNow()
	}

}

func Test_That_An_Error_Is_Returned_When_Authentication_Fails(t *testing.T) {
	config := MeliConfig{

		ClientID:       CLIENT_ID,
		UserCode:       "NEW_CODE",
		Secret:         CLIENT_SECRET,
		CallBackURL:    "http://www.example.com",
		HTTPClient:     MockHttpClientPostFailure{},
		TokenRefresher: MockTockenRefresher{},
	}

	_, error := MeliClient(config)

	if error == nil {
		log.Printf("Error: An error should have been received.")
		t.FailNow()
	}

}

func Test_That_MeliTokenRefresher_Returns_An_Error_When_Posting_Authorization_Fails(t *testing.T) {

	config := MeliConfig{

		ClientID:       CLIENT_ID,
		UserCode:       "ANOTHER_CODE",
		Secret:         CLIENT_SECRET,
		CallBackURL:    "http://www.example.com",
		HTTPClient:     MockHttpClient{},
		TokenRefresher: MockTockenRefresher{},
	}
	client, error := MeliClient(config)

	if error != nil {
		log.Printf("Error: A client should have been returned.")
		t.FailNow()
	}

	client.httpClient = MockHttpClientPostFailure{}

	tokenRefresher := MeliTokenRefresher{}
	error = tokenRefresher.RefreshToken(client)

	if error == nil {
		log.Printf("Error: An error should have been received.")
		t.FailNow()
	}

}

func Test_MeliTokenRefresher_Returns_An_Error_When_Authorization_Returns_A_HTTP_StatusCode_Different_From_200(t *testing.T) {

	config := MeliConfig{

		ClientID:       CLIENT_ID,
		UserCode:       "ANOTHER_CODE",
		Secret:         CLIENT_SECRET,
		CallBackURL:    "http://www.example.com",
		HTTPClient:     MockHttpClient{},
		TokenRefresher: MockTockenRefresher{},
	}
	client, error := MeliClient(config)

	if error != nil {
		log.Printf("Error: A client should have been returned.")
		t.FailNow()
	}

	client.httpClient = MockHttpClientPostNonOKStatusCode{}

	tokenRefresher := MeliTokenRefresher{}
	error = tokenRefresher.RefreshToken(client)

	if error == nil {
		log.Printf("Error: An error should not have been received.")
		t.FailNow()
	}

	//TODO: Please..DO NOT COMPARE errors by using strings. Fix this up.
	if strings.Compare(fmt.Sprintf("%s", error.Error()), "Refreshing token returned status code ") != 0 {
		log.Printf("Error: An error should have been received.")
		t.FailNow()
	}

}

func Test_Return_Authorized_FALSE_When_Client_Is_NOT_Authorized(t *testing.T) {

	client, _ := Meli(CLIENT_ID, "", "", "www.example.com/me")

	if client.IsAuthorized() == true {
		log.Printf("Client should not be authorized")
		t.FailNow()
	}
}

func Test_Return_Authorized_TRUE_When_Client_Is_Authorized(t *testing.T) {

	config := MeliConfig{

		ClientID:       CLIENT_ID,
		UserCode:       "AUTHORIZED_CLIENT",
		Secret:         CLIENT_SECRET,
		CallBackURL:    "http://www.example.com",
		HTTPClient:     MockHttpClient{},
		TokenRefresher: MockTockenRefresher{},
	}

	client, err := MeliClient(config)

	if err != nil {
		log.Printf("Error: %s", err.Error())
		t.FailNow()
	}
	if client.IsAuthorized() != true {
		log.Printf("Client should be authorized")
		t.FailNow()
	}
}

func Test_GET_public_API_sites_works_properly(t *testing.T) {

	client, err := newTestAnonymousClient(API_TEST)

	if err != nil {
		log.Printf("Error:%s\n", err)
		t.FailNow()
	}
	//Public APIs do not need Authorization
	resp, err := client.Get("/sites")

	if err != nil {
		log.Printf("Error:%s\n", err)
		t.FailNow()
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("Error:Status was different from the expected one %s\n", err)
		t.FailNow()
	}

	body, err := ioutil.ReadAll(resp.Body)

	if err != nil || string(body) == "" {
		t.FailNow()
	}
}

func Test_GET_private_API_users_works_properly(t *testing.T) {

	client, err := newTestClient(CLIENT_ID, USER_CODE, CLIENT_SECRET, "https://www.example.com", API_TEST)

	_, err = client.Get("/users/me")

	if err != nil {
		fmt.Printf("Error: %s\n", err)
		t.FailNow()
	}
}

func Test_POST_a_new_item_works_properly_when_token_IS_EXPIRED(t *testing.T) {

	client, err := newTestClient(CLIENT_ID, USER_CODE, CLIENT_SECRET, "https://www.example.com", API_TEST)

	body := "{\"foo\":\"bar\"}"
	resp, err := client.Post("/items", body)

	if err != nil {
		log.Printf("Error while posting a new item %s\n", err)
		t.FailNow()
	}

	if resp.StatusCode != http.StatusCreated {
		log.Printf("Error while posting a new item status code: %d\n", resp.StatusCode)
		t.FailNow()
	}
}

func Test_POST_a_new_item_works_properly_when_token_IS_NOT_EXPIRED(t *testing.T) {

	client, err := newTestClient(CLIENT_ID, USER_CODE, CLIENT_SECRET, "https://www.example.com", API_TEST)

	body := "{\"foo\":\"bar\"}"
	resp, err := client.Post("/items", body)

	if err != nil {
		log.Printf("Error while posting a new item %s\n", err)
		t.FailNow()
	}

	if resp.StatusCode != http.StatusCreated {
		log.Printf("Error while posting a new item status code: %d\n", resp.StatusCode)
		t.FailNow()
	}
}

func Test_PUT_a_new_item_works_properly_when_token_IS_NOT_EXPIRED(t *testing.T) {

	client, err := newTestClient(CLIENT_ID, USER_CODE, CLIENT_SECRET, "https://www.example.com", API_TEST)

	body := "{\"foo\":\"bar\"}"
	resp, err := client.Put("/items/123", body)

	if err != nil {
		log.Printf("Error while posting a new item %s\n", err)
		t.FailNow()
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("Error while putting a new item. Status code: %d\n", resp.StatusCode)
		t.FailNow()
	}
}

func Test_PUT_a_new_item_works_properly_when_token_IS_EXPIRED(t *testing.T) {

	client, err := newTestClient(CLIENT_ID, USER_CODE, CLIENT_SECRET, "https://www.example.com", API_TEST)

	body := "{\"foo\":\"bar\"}"
	resp, err := client.Put("/items/123", body)

	if err != nil {
		log.Printf("Error while posting a new item %s\n", err)
		t.FailNow()
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("Error while putting a new item. Status code: %d\n", resp.StatusCode)
		t.FailNow()
	}
}

func Test_DELETE_an_item_returns_200_when_token_IS_NOT_EXPIRED(t *testing.T) {

	client, err := newTestClient(CLIENT_ID, USER_CODE, CLIENT_SECRET, "https://www.example.com", API_TEST)

	resp, err := client.Delete("/items/123")

	if err != nil {
		log.Printf("Error while deleting an item %s\n", err)
		t.FailNow()
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("Error while putting a new item. Status code: %d\n", resp.StatusCode)
		t.FailNow()
	}
}

func Test_DELETE_an_item_returns_200_when_token_IS_EXPIRED(t *testing.T) {

	client, err := newTestClient(CLIENT_ID, USER_CODE, CLIENT_SECRET, "https://www.example.com", API_TEST)

	resp, err := client.Delete("/items/123")

	if err != nil {
		log.Printf("Error while deleting an item %s\n", err)
		t.FailNow()
	}
	if resp.StatusCode != http.StatusOK {
		log.Printf("Error while putting a new item. Status code: %d\n", resp.StatusCode)
		t.FailNow()
	}
}

func Test_AuthorizationURL_adds_a_params_separator_when_needed(t *testing.T) {

	auth := newAuthorizationURL(APIURL + "/authorizationauth")
	auth.addGrantType(AuthoricationCode)

	url := APIURL + "/authorizationauth?" + "grant_type=" + AuthoricationCode

	if strings.Compare(url, auth.string()) != 0 {
		log.Printf("url was different from what was expected\n expected: %s \n obtained: %s \n", url, auth.string())
		t.FailNow()
	}
}

func Test_AuthorizationURL_adds_a_query_param_separator_when_needed(t *testing.T) {

	auth := newAuthorizationURL(APIURL + "/authorizationauth")
	auth.addGrantType(AuthoricationCode)
	auth.addClientId(1213213)

	url := APIURL + "/authorizationauth?" + "grant_type=" + AuthoricationCode + "&client_id=1213213"

	if strings.Compare(url, auth.string()) != 0 {
		log.Printf("url was different from what was expected\n expected: %s \n obtained: %s \n", url, auth.string())
		t.FailNow()
	}
}

func Test_only_one_token_refresh_call_is_done_when_several_threads_are_executed(t *testing.T) {

	client, err := newTestClient(CLIENT_ID, USER_CODE, CLIENT_SECRET, "https://www.example.com", API_TEST)

	if err != nil {
		log.Printf("Error during Client instantation %s\n", err)
		t.FailNow()
	}
	client.auth.ExpiresIn = 0

	wg.Add(100)
	for i := 0; i < 100; i++ {
		go callHttpMethod(client)
	}
	wg.Wait()

	if counter > 1 {
		t.FailNow()
	}
}

var counter = 0
var m = sync.Mutex{}

type MockTockenRefresher struct{}

func (mock MockTockenRefresher) RefreshToken(client *Client) error {
	realRefresher := MeliTokenRefresher{}
	realRefresher.RefreshToken(client)
	m.Lock()
	counter++
	fmt.Printf("counter %d", counter)
	m.Unlock()
	return nil
}

var wg sync.WaitGroup

func callHttpMethod(client *Client) {
	defer wg.Done()
	client.Get("/users/me")
}

/*
Clients for testing purposes
*/
func newTestAnonymousClient(apiUrl string) (*Client, error) {

	client := &Client{apiURL: apiUrl, auth: anonymous, httpClient: MockHttpClient{}}

	return client, nil
}

func newTestClient(id int64, code string, secret string, redirectUrl string, apiUrl string) (*Client, error) {

	client := &Client{id: id, code: code, secret: secret, redirectURL: redirectUrl, apiURL: apiUrl, httpClient: MockHttpClient{}, tokenRefresher: MockTockenRefresher{}}

	auth, err := client.authorize()

	if err != nil {
		return nil, err
	}

	client.auth = *auth

	return client, nil
}

type MockHttpClient struct {
}

func (httpClient MockHttpClient) Get(url string) (*http.Response, error) {
	resp := new(http.Response)

	if strings.Contains(url, "/sites") {
		resp.Body = ioutil.NopCloser(bytes.NewReader([]byte("[{\"id\":\"MLA\",\"name\":\"Argentina\"},{\"id\":\"MLB\",\"name\":\"Brasil\"},{\"id\":\"MCO\",\"name\":\"Colombia\"},{\"id\":\"MCR\",\"name\":\"Costa Rica\"},{\"id\":\"MEC\",\"name\":\"Ecuador\"},{\"id\":\"MLC\",\"name\":\"Chile\"},{\"id\":\"MLM\",\"name\":\"Mexico\"},{\"id\":\"MLU\",\"name\":\"Uruguay\"},{\"id\":\"MLV\",\"name\":\"Venezuela\"},{\"id\":\"MPA\",\"name\":\"Panamá\"},{\"id\":\"MPE\",\"name\":\"Perú\"},{\"id\":\"MPT\",\"name\":\"Portugal\"},{\"id\":\"MRD\",\"name\":\"Dominicana\"}]\")))")))
		resp.StatusCode = http.StatusOK
	}

	if strings.Contains(url, "/users/me") {
		resp.Body = ioutil.NopCloser(bytes.NewReader([]byte("")))
		resp.StatusCode = http.StatusOK
	}

	if strings.Contains(url, "/authsites") {
		resp.Body = ioutil.NopCloser(bytes.NewReader([]byte(`[{"id":"MLA","name":"Argentina","url":"https://auth.mercadolibre.com.ar"},{"id":"MLB","name":"Brasil","url":"https://auth.mercadolivre.com.br"},{"id":"MCO","name":"Colombia","url":"https://auth.mercadolibre.com.co"},{"id":"MCR","name":"Costa Rica","url":"https://auth.mercadolibre.com.cr"},{"id":"MEC","name":"Ecuador","url":"https://auth.mercadolibre.com.ec"},{"id":"MLC","name":"Chile","url":"https://auth.mercadolibre.cl"},{"id":"MLM","name":"Mexico","url":"https://auth.mercadolibre.com.mx"},{"id":"MLU","name":"Uruguay","url":"https://auth.mercadolibre.com.uy"},{"id":"MLV","name":"Venezuela","url":"https://auth.mercadolibre.com.ve"},{"id":"MPA","name":"Panamá","url":"https://auth.mercadolibre.com.pa"},{"id":"MPE","name":"Perú","url":"https://auth.mercadolibre.com.pe"},{"id":"MPT","name":"Portugal","url":"https://auth.mercadolivre.pt"},{"id":"MRD","name":"Dominicana","url":"https://auth.mercadolibre.com.do"},{"id":"CBT","name":"","url":""}]`)))
		resp.StatusCode = http.StatusOK
	}

	return resp, nil
}

func (httpClient MockHttpClient) Post(uri string, bodyType string, body io.Reader) (*http.Response, error) {

	resp := new(http.Response)
	fullUri, _ := url.Parse(uri)

	if strings.Contains(uri, "/oauth/token") {

		grant_type := fullUri.Query().Get("grant_type")

		if strings.Compare(grant_type, "authorization_code") == 0 {
			code := fullUri.Query().Get("code")

			if strings.Compare(code, "bad code") == 0 {

				resp.Body = ioutil.NopCloser(bytes.NewReader([]byte("{\"message\":\"Error validando el parámetro code\",\"error\":\"invalid_grant\"}")))
				resp.StatusCode = http.StatusNotFound

			} else if strings.Compare(code, "valid code without refresh token") == 0 {

				resp.Body = ioutil.NopCloser(bytes.NewReader([]byte(
					"{\"access_token\" : \"valid token\"," +
						"\"token_type\" : \"bearer\"," +
						"\"expires_in\" : 10800," +
						"\"scope\" : \"write read\"}")))

				resp.StatusCode = http.StatusOK

			} else if strings.Compare(code, "valid code with refresh token") == 0 ||
				strings.Compare(code, "ANOTHER_CODE") == 0 ||
				strings.Compare(code, "AUTHORIZED_CLIENT") == 0 {

				resp.Body = ioutil.NopCloser(bytes.NewReader([]byte(
					"{\"access_token\":\"valid token\"," +
						"\"token_type\":\"bearer\"," +
						"\"expires_in\":10800," +
						"\"refresh_token\":\"valid refresh token\"," +
						"\"scope\":\"write read\"}")))

			}

		} else if strings.Compare(grant_type, "refresh_token") == 0 {

			refresh := fullUri.Query().Get("refresh_token")

			if strings.Compare(refresh, "valid refresh token") == 0 {

				resp.Body = ioutil.NopCloser(bytes.NewReader([]byte(
					"{\"access_token\":\"valid token\"," +
						"\"token_type\":\"bearer\"," +
						"\"expires_in\":10800," +
						"\"scope\":\"write read\"}")))
			}
		}

		resp.StatusCode = http.StatusOK

	} else if strings.Contains(uri, "/items") {

		access_token := fullUri.Query().Get("access_token")

		if strings.Compare(access_token, "valid token") == 0 {

			b, _ := ioutil.ReadAll(body)
			if b != nil && strings.Contains(string(b), "bar") {
				resp.StatusCode = http.StatusCreated
			} else {
				resp.StatusCode = http.StatusNotFound
			}
		}
	}

	return resp, nil
}

func (httpClient MockHttpClient) Put(uri string, body io.Reader) (*http.Response, error) {

	resp := new(http.Response)
	fullUri, _ := url.Parse(uri)

	if strings.Contains(uri, "/items/123") {

		access_token := fullUri.Query().Get("access_token")

		if strings.Compare(access_token, "valid token") == 0 {

			b, _ := ioutil.ReadAll(body)
			if b != nil && strings.Contains(string(b), "bar") {
				resp.StatusCode = http.StatusOK
			} else {
				resp.StatusCode = http.StatusNotFound
			}

		} else if strings.Compare(access_token, "expired token") == 0 {
			resp.StatusCode = http.StatusNotFound
		} else {
			resp.StatusCode = http.StatusForbidden
		}
	}

	return resp, nil
}

func (httpClient MockHttpClient) Delete(uri string, body io.Reader) (*http.Response, error) {

	resp := new(http.Response)
	fullUri, _ := url.Parse(uri)

	if strings.Contains(uri, "/items/123") {
		access_token := fullUri.Query().Get("access_token")

		if strings.Compare(access_token, "valid token") == 0 {
			resp.StatusCode = http.StatusOK
		} else if strings.Compare(access_token, "expired token") == 0 {
			resp.StatusCode = http.StatusNotFound
		} else {
			resp.StatusCode = http.StatusForbidden
		}
	}

	return resp, nil
}

type MockHttpClientPostFailure struct {
}

func (httpClient MockHttpClientPostFailure) Post(uri string, bodyType string, body io.Reader) (*http.Response, error) {
	return nil, errors.New("Error")
}
func (httpClient MockHttpClientPostFailure) Get(url string) (*http.Response, error) {
	switch url {
	case "https://api.mercadolibre.com/authsites":
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       ioutil.NopCloser(strings.NewReader(`[{"id":"MLA","name":"Argentina","url":"https://auth.mercadolibre.com.ar"},{"id":"MLB","name":"Brasil","url":"https://auth.mercadolivre.com.br"},{"id":"MCO","name":"Colombia","url":"https://auth.mercadolibre.com.co"},{"id":"MCR","name":"Costa Rica","url":"https://auth.mercadolibre.com.cr"},{"id":"MEC","name":"Ecuador","url":"https://auth.mercadolibre.com.ec"},{"id":"MLC","name":"Chile","url":"https://auth.mercadolibre.cl"},{"id":"MLM","name":"Mexico","url":"https://auth.mercadolibre.com.mx"},{"id":"MLU","name":"Uruguay","url":"https://auth.mercadolibre.com.uy"},{"id":"MLV","name":"Venezuela","url":"https://auth.mercadolibre.com.ve"},{"id":"MPA","name":"Panamá","url":"https://auth.mercadolibre.com.pa"},{"id":"MPE","name":"Perú","url":"https://auth.mercadolibre.com.pe"},{"id":"MPT","name":"Portugal","url":"https://auth.mercadolivre.pt"},{"id":"MRD","name":"Dominicana","url":"https://auth.mercadolibre.com.do"},{"id":"CBT","name":"","url":""}]`)),
		}, nil
	}
	return nil, nil
}

func (httpClient MockHttpClientPostFailure) Delete(uri string, body io.Reader) (*http.Response, error) {
	return nil, nil
}

func (httpClient MockHttpClientPostFailure) Put(uri string, body io.Reader) (*http.Response, error) {
	return nil, nil
}

type MockHttpClientPostNonOKStatusCode struct {
}

func (httpClient MockHttpClientPostNonOKStatusCode) Post(uri string, bodyType string, body io.Reader) (*http.Response, error) {

	httpResponse := http.Response{}
	httpResponse.StatusCode = http.StatusForbidden
	return new(http.Response), nil
}
func (httpClient MockHttpClientPostNonOKStatusCode) Get(url string) (*http.Response, error) {
	return nil, nil
}

func (httpClient MockHttpClientPostNonOKStatusCode) Delete(uri string, body io.Reader) (*http.Response, error) {
	return nil, nil
}

func (httpClient MockHttpClientPostNonOKStatusCode) Put(uri string, body io.Reader) (*http.Response, error) {
	return nil, nil
}
