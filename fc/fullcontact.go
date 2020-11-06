package fullcontact

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"
)

func (fcClient *fullContactClient) newHttpRequest(url string, reqBytes []byte) (*http.Request, error) {
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(reqBytes))
	if err != nil {
		return nil, err
	}
	for k, v := range fcClient.headers {
		req.Header.Add(k, v)
	}
	req.Header.Add("Authorization", "Bearer "+fcClient.credentialsProvider.getApiKey())
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("User-Agent", userAgent)
	return req, nil

}

func (fcClient *fullContactClient) newHttpGetRequest(url string, query string) (*http.Request, error) {
	req, err := http.NewRequest("GET", url+"?"+query, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range fcClient.headers {
		req.Header.Add(k, v)
	}
	req.Header.Add("Authorization", "Bearer "+fcClient.credentialsProvider.getApiKey())
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("User-Agent", userAgent)
	return req, nil

}

func (fcClient *fullContactClient) do(url string, reqBytes []byte, ch chan *APIResponse) {
	var req *http.Request
	var err error
	//construct for HTTP GET requests
	if url == emailVerificationUrl {
		req, err = fcClient.newHttpGetRequest(url, "email="+string(reqBytes))
	} else {
		req, err = fcClient.newHttpRequest(url, reqBytes)
	}
	if err != nil {
		sendToChannel(ch, nil, url, err)
	}

	resp, err := fcClient.httpClient.Do(req) //first attempt

	if err != nil {
		fcClient.autoRetry(ch, err, resp, 0, url, reqBytes)
	} else if resp != nil && !fcClient.retryHandler.ShouldRetry(resp.StatusCode) {
		sendToChannel(ch, resp, url, nil)
	} else {
		fcClient.autoRetry(ch, nil, resp, 0, url, reqBytes)
	}
}

func (fcClient *fullContactClient) autoRetry(ch chan *APIResponse, err error, resp *http.Response, retryAttemptsDone int, url string, reqBytes []byte) {
	if retryAttemptsDone < min(fcClient.retryHandler.RetryAttempts(), 5) {
		retryAttemptsDone++
		time.Sleep(time.Duration(fcClient.retryHandler.RetryDelayMillis()*(1<<(retryAttemptsDone-1))) * time.Millisecond)
		var req *http.Request
		var err error
		if url == emailVerificationUrl {
			req, err = fcClient.newHttpGetRequest(url, "email="+string(reqBytes))
		} else {
			req, err = fcClient.newHttpRequest(url, reqBytes)
		}
		if err != nil {
			sendToChannel(ch, nil, url, err)
		}
		resp, err = fcClient.httpClient.Do(req)
		if err != nil {
			fcClient.autoRetry(ch, err, resp, retryAttemptsDone, url, reqBytes)
		} else if resp != nil && !fcClient.retryHandler.ShouldRetry(resp.StatusCode) {
			sendToChannel(ch, resp, url, nil)
		} else {
			fcClient.autoRetry(ch, nil, resp, retryAttemptsDone, url, reqBytes)
		}
	} else if err != nil {
		sendToChannel(ch, nil, url, err)
	} else {
		sendToChannel(ch, resp, url, nil)
	}

}

func sendToChannel(ch chan *APIResponse, response *http.Response, url string, err error) {
	apiResponse := &APIResponse{
		RawHttpResponse: response,
		Err:             err,
	}

	if response != nil {
		//For Testing Purposes
		testType := response.Header.Get(FCGoClientTestType)
		if isPopulated(testType) {
			url = testType
		}

		switch url {
		case personEnrichUrl:
			setPersonResponse(apiResponse)
		case companyEnrichUrl:
			setCompanyResponse(apiResponse)
		case companySearchUrl:
			setCompanySearchResponse(apiResponse)
		case identityMapUrl, identityResolveUrl, identityDeleteUrl:
			setResolveResponse(apiResponse)
		case identityResolveWithTagsUrl:
			setResolveResponseWithTags(apiResponse)
		case tagsCreateUrl, tagsGetUrl, tagsDeleteUrl:
			setTagsResponse(apiResponse)
		case audienceCreateUrl, audienceDownloadUrl:
			setAudienceResponse(apiResponse)
		case emailVerificationUrl:
			setEmailVerificationResponse(apiResponse)
		}
	}
	ch <- apiResponse
	return
}

/* FullContact V3 Person Enrich API, takes an PersonRequest and returns a channel of type APIResponse.
Request is converted to JSON and sends a Asynchronous request */
func (fcClient *fullContactClient) PersonEnrich(personRequest *PersonRequest) chan *APIResponse {
	ch := make(chan *APIResponse)
	if personRequest == nil {
		go sendToChannel(ch, nil, "", NewFullContactError("Person Request can't be nil"))
		return ch
	}
	reqBytes, err := json.Marshal(personRequest)

	if err != nil {
		go sendToChannel(ch, nil, "", err)
		return ch
	}
	// Send Asynchronous Request in Goroutine
	go fcClient.do(personEnrichUrl, reqBytes, ch)
	return ch
}

/* FullContact V3 Company Enrich API, takes an CompanyRequest and returns a channel of type APIResponse.
Request is converted to JSON and sends a Asynchronous request */
func (fcClient *fullContactClient) CompanyEnrich(companyRequest *CompanyRequest) chan *APIResponse {
	ch := make(chan *APIResponse)
	if companyRequest == nil {
		go sendToChannel(ch, nil, "", NewFullContactError("Company Request can't be nil"))
		return ch
	}
	err := validateForCompanyEnrich(companyRequest)
	if err != nil {
		go sendToChannel(ch, nil, "", err)
		return ch
	}
	reqBytes, err := json.Marshal(companyRequest)

	if err != nil {
		go sendToChannel(ch, nil, "", err)
		return ch
	}
	// Send Asynchronous Request in Goroutine
	go fcClient.do(companyEnrichUrl, reqBytes, ch)
	return ch
}

/* FullContact V3 Company Search API, takes an CompanyRequest and returns a channel of type APIResponse.
Request is converted to JSON and sends a Asynchronous request */
func (fcClient *fullContactClient) CompanySearch(companyRequest *CompanyRequest) chan *APIResponse {
	ch := make(chan *APIResponse)
	if companyRequest == nil {
		go sendToChannel(ch, nil, "", NewFullContactError("Company Request can't be nil"))
		return ch
	}
	err := validateForCompanySearch(companyRequest)
	if err != nil {
		go sendToChannel(ch, nil, "", err)
		return ch
	}
	reqBytes, err := json.Marshal(companyRequest)

	if err != nil {
		go sendToChannel(ch, nil, "", err)
		return ch
	}
	// Send Asynchronous Request in Goroutine
	go fcClient.do(companySearchUrl, reqBytes, ch)
	return ch
}

/* Resolve
FullContact Resolve API - IdentityMap, takes an ResolveRequest and returns a channel of type APIResponse.
Request is converted to JSON and sends a Asynchronous request */
func (fcClient *fullContactClient) IdentityMap(resolveRequest *ResolveRequest) chan *APIResponse {
	ch := make(chan *APIResponse)
	if resolveRequest == nil {
		go sendToChannel(ch, nil, "", NewFullContactError("Resolve Request can't be nil"))
		return ch
	}
	err := validateForIdentityMap(resolveRequest)
	if err != nil {
		go sendToChannel(ch, nil, "", err)
		return ch
	}
	return fcClient.resolveRequest(ch, resolveRequest, identityMapUrl)
}

/* Resolve
FullContact Resolve API - IdentityResolve, takes an ResolveRequest and returns a channel of type APIResponse.
Request is converted to JSON and sends a Asynchronous request */
func (fcClient *fullContactClient) IdentityResolve(resolveRequest *ResolveRequest) chan *APIResponse {
	ch := make(chan *APIResponse)
	if resolveRequest == nil {
		go sendToChannel(ch, nil, "", NewFullContactError("Resolve Request can't be nil"))
		return ch
	}
	err := validateForIdentityResolve(resolveRequest)
	if err != nil {
		go sendToChannel(ch, nil, "", err)
		return ch
	}
	return fcClient.resolveRequest(ch, resolveRequest, identityResolveUrl)
}

/* Resolve
FullContact Resolve API - IdentityResolve with Tags in response, takes an ResolveRequest and returns a channel of type APIResponse.
Request is converted to JSON and sends a Asynchronous request */
func (fcClient *fullContactClient) IdentityResolveWithTags(resolveRequest *ResolveRequest) chan *APIResponse {
	ch := make(chan *APIResponse)
	if resolveRequest == nil {
		go sendToChannel(ch, nil, "", NewFullContactError("Resolve Request can't be nil"))
		return ch
	}
	err := validateForIdentityResolve(resolveRequest)
	if err != nil {
		go sendToChannel(ch, nil, "", err)
		return ch
	}
	return fcClient.resolveRequest(ch, resolveRequest, identityResolveWithTagsUrl)
}

/* Resolve
FullContact Resolve API - IdentityDelete, takes an ResolveRequest and returns a channel of type APIResponse.
Request is converted to JSON and sends a Asynchronous request */
func (fcClient *fullContactClient) IdentityDelete(resolveRequest *ResolveRequest) chan *APIResponse {
	ch := make(chan *APIResponse)
	if resolveRequest == nil {
		go sendToChannel(ch, nil, "", NewFullContactError("Resolve Request can't be nil"))
		return ch
	}
	err := validateForIdentityDelete(resolveRequest)
	if err != nil {
		go sendToChannel(ch, nil, "", err)
		return ch
	}
	return fcClient.resolveRequest(ch, resolveRequest, identityDeleteUrl)
}

func (fcClient *fullContactClient) resolveRequest(ch chan *APIResponse, resolveRequest *ResolveRequest, url string) chan *APIResponse {
	reqBytes, err := json.Marshal(resolveRequest)

	if err != nil {
		go sendToChannel(ch, nil, "", err)
		return ch
	}
	// Send Asynchronous Request in Goroutine
	go fcClient.do(url, reqBytes, ch)
	return ch
}

/* FullContact API for adding/creating tags for any recordId in your PIC, takes a TagsRequest and returns a channel of type APIResponse.
Request is converted to JSON and sends a Asynchronous request */
func (fcClient *fullContactClient) TagsCreate(tagsRequest *TagsRequest) chan *APIResponse {
	ch := make(chan *APIResponse)
	if tagsRequest == nil {
		go sendToChannel(ch, nil, "", NewFullContactError("Tags Request can't be nil"))
		return ch
	}
	reqBytes, err := json.Marshal(tagsRequest)

	if err != nil {
		go sendToChannel(ch, nil, "", err)
		return ch
	}
	// Send Asynchronous Request in Goroutine
	go fcClient.do(tagsCreateUrl, reqBytes, ch)
	return ch
}

/* FullContact API for getting all tags for any recordId in your PIC, takes a 'recordId' and returns a channel of type APIResponse.
Request is converted to JSON and sends a Asynchronous request */
func (fcClient *fullContactClient) TagsGet(recordId string) chan *APIResponse {
	ch := make(chan *APIResponse)
	if !isPopulated(recordId) {
		go sendToChannel(ch, nil, "", NewFullContactError("recordId can't be nil"))
		return ch
	}
	reqBytes := []byte("{\"recordId\":\"" + recordId + "\"}")

	// Send Asynchronous Request in Goroutine
	go fcClient.do(tagsGetUrl, reqBytes, ch)
	return ch
}

/* FullContact API for deleting any tag(s) for any recordId in your PIC, takes a TagsRequest and returns a channel of type APIResponse.
Request is converted to JSON and sends a Asynchronous request */
func (fcClient *fullContactClient) TagsDelete(tagsRequest *TagsRequest) chan *APIResponse {
	ch := make(chan *APIResponse)
	if tagsRequest == nil {
		go sendToChannel(ch, nil, "", NewFullContactError("Tags Request can't be nil"))
		return ch
	}
	reqBytes, err := json.Marshal(tagsRequest)

	if err != nil {
		go sendToChannel(ch, nil, "", err)
		return ch
	}
	// Send Asynchronous Request in Goroutine
	go fcClient.do(tagsDeleteUrl, reqBytes, ch)
	return ch
}

/* FullContact API for creating Audience based on tags from your PIC, takes a AudienceRequest and returns a channel of type APIResponse.
Request is converted to JSON and sends a Asynchronous request */
func (fcClient *fullContactClient) AudienceCreate(audienceRequest *AudienceRequest) chan *APIResponse {
	ch := make(chan *APIResponse)
	if audienceRequest == nil {
		go sendToChannel(ch, nil, "", NewFullContactError("Audience Request can't be nil"))
		return ch
	}
	reqBytes, err := json.Marshal(audienceRequest)

	if err != nil {
		go sendToChannel(ch, nil, "", err)
		return ch
	}
	// Send Asynchronous Request in Goroutine
	go fcClient.do(audienceCreateUrl, reqBytes, ch)
	return ch
}

/* FullContact Email Verification API, takes an 'email' as string and returns a channel of type APIResponse.
Request is converted to JSON and sends a Asynchronous request */
func (fcClient *fullContactClient) EmailVerification(email string) chan *APIResponse {
	ch := make(chan *APIResponse)
	if !isPopulated(email) {
		go sendToChannel(ch, nil, "", NewFullContactError("email can't be nil"))
		return ch
	}
	reqBytes := []byte(email)

	// Send Asynchronous Request in Goroutine
	go fcClient.do(emailVerificationUrl, reqBytes, ch)
	return ch
}

func setPersonResponse(apiResponse *APIResponse) {
	bodyBytes, err := ioutil.ReadAll(apiResponse.RawHttpResponse.Body)
	defer apiResponse.RawHttpResponse.Body.Close()
	if err != nil {
		apiResponse.Err = err
		return
	}
	var person PersonResp
	if isPopulated(string(bodyBytes)) {
		err = json.Unmarshal(bodyBytes, &person)
		if err != nil {
			apiResponse.Err = err
			return
		}
	}
	apiResponse.Status = apiResponse.RawHttpResponse.Status
	apiResponse.StatusCode = apiResponse.RawHttpResponse.StatusCode
	apiResponse.IsSuccessful = (apiResponse.StatusCode == 200) || (apiResponse.StatusCode == 202) || (apiResponse.StatusCode == 404)
	apiResponse.PersonResponse = &person
}

func setCompanyResponse(apiResponse *APIResponse) {
	bodyBytes, err := ioutil.ReadAll(apiResponse.RawHttpResponse.Body)
	defer apiResponse.RawHttpResponse.Body.Close()
	if err != nil {
		apiResponse.Err = err
		return
	}
	var companyResponse CompanyResponse
	if isPopulated(string(bodyBytes)) {
		err = json.Unmarshal(bodyBytes, &companyResponse)
		if err != nil {
			apiResponse.Err = err
			return
		}
	}
	apiResponse.Status = apiResponse.RawHttpResponse.Status
	apiResponse.StatusCode = apiResponse.RawHttpResponse.StatusCode
	apiResponse.IsSuccessful = (apiResponse.StatusCode == 200) || (apiResponse.StatusCode == 202) || (apiResponse.StatusCode == 404)
	apiResponse.CompanyResponse = &companyResponse
}

func setCompanySearchResponse(apiResponse *APIResponse) {
	bodyBytes, err := ioutil.ReadAll(apiResponse.RawHttpResponse.Body)
	defer apiResponse.RawHttpResponse.Body.Close()
	if err != nil {
		apiResponse.Err = err
		return
	}
	var companySearchResponse []*CompanySearchResponse
	if isPopulated(string(bodyBytes)) {
		err = json.Unmarshal(bodyBytes, &companySearchResponse)
		if err != nil {
			apiResponse.Err = err
			return
		}
	}
	apiResponse.Status = apiResponse.RawHttpResponse.Status
	apiResponse.StatusCode = apiResponse.RawHttpResponse.StatusCode
	apiResponse.IsSuccessful = (apiResponse.StatusCode == 200) || (apiResponse.StatusCode == 202) || (apiResponse.StatusCode == 404)
	apiResponse.CompanySearchResponse = companySearchResponse
}

func setResolveResponse(apiResponse *APIResponse) {
	bodyBytes, err := ioutil.ReadAll(apiResponse.RawHttpResponse.Body)
	defer apiResponse.RawHttpResponse.Body.Close()
	if err != nil {
		apiResponse.Err = err
		return
	}
	var resolveResponse ResolveResponse
	if isPopulated(string(bodyBytes)) {
		err = json.Unmarshal(bodyBytes, &resolveResponse)
		if err != nil {
			apiResponse.Err = err
			return
		}
	}
	apiResponse.Status = apiResponse.RawHttpResponse.Status
	apiResponse.StatusCode = apiResponse.RawHttpResponse.StatusCode
	apiResponse.IsSuccessful = (apiResponse.StatusCode == 200) || (apiResponse.StatusCode == 204) || (apiResponse.StatusCode == 404)
	apiResponse.ResolveResponse = &resolveResponse
}

func setResolveResponseWithTags(apiResponse *APIResponse) {
	bodyBytes, err := ioutil.ReadAll(apiResponse.RawHttpResponse.Body)
	defer apiResponse.RawHttpResponse.Body.Close()
	if err != nil {
		apiResponse.Err = err
		return
	}
	var resolveResponse ResolveResponseWithTags
	if isPopulated(string(bodyBytes)) {
		err = json.Unmarshal(bodyBytes, &resolveResponse)
		if err != nil {
			apiResponse.Err = err
			return
		}
	}
	apiResponse.Status = apiResponse.RawHttpResponse.Status
	apiResponse.StatusCode = apiResponse.RawHttpResponse.StatusCode
	apiResponse.IsSuccessful = (apiResponse.StatusCode == 200) || (apiResponse.StatusCode == 204) || (apiResponse.StatusCode == 404)
	apiResponse.ResolveResponseWithTags = &resolveResponse
}

func setTagsResponse(apiResponse *APIResponse) {
	bodyBytes, err := ioutil.ReadAll(apiResponse.RawHttpResponse.Body)
	defer apiResponse.RawHttpResponse.Body.Close()
	if err != nil {
		apiResponse.Err = err
		return
	}
	var tagsResponse TagsResponse
	if isPopulated(string(bodyBytes)) {
		err = json.Unmarshal(bodyBytes, &tagsResponse)
		if err != nil {
			apiResponse.Err = err
			return
		}
	}
	apiResponse.Status = apiResponse.RawHttpResponse.Status
	apiResponse.StatusCode = apiResponse.RawHttpResponse.StatusCode
	apiResponse.IsSuccessful = (apiResponse.StatusCode == 200) || (apiResponse.StatusCode == 204) || (apiResponse.StatusCode == 404)
	apiResponse.TagsResponse = &tagsResponse
}

func setAudienceResponse(apiResponse *APIResponse) {
	bodyBytes, err := ioutil.ReadAll(apiResponse.RawHttpResponse.Body)
	defer apiResponse.RawHttpResponse.Body.Close()
	if err != nil {
		apiResponse.Err = err
		return
	}
	var audienceResponse AudienceResponse
	if isPopulated(string(bodyBytes)) {
		err = json.Unmarshal(bodyBytes, &audienceResponse)
		if err != nil {
			apiResponse.Err = err
			return
		}
	}
	apiResponse.Status = apiResponse.RawHttpResponse.Status
	apiResponse.StatusCode = apiResponse.RawHttpResponse.StatusCode
	apiResponse.IsSuccessful = (apiResponse.StatusCode == 200) || (apiResponse.StatusCode == 202) || (apiResponse.StatusCode == 404)
	apiResponse.AudienceResponse = &audienceResponse
}

func setEmailVerificationResponse(apiResponse *APIResponse) {
	bodyBytes, err := ioutil.ReadAll(apiResponse.RawHttpResponse.Body)
	defer apiResponse.RawHttpResponse.Body.Close()
	if err != nil {
		apiResponse.Err = err
		return
	}
	var emailResponse EmailVerificationResponse
	if isPopulated(string(bodyBytes)) {
		err = json.Unmarshal(bodyBytes, &emailResponse)
		if err != nil {
			apiResponse.Err = err
			return
		}
	}
	apiResponse.Status = apiResponse.RawHttpResponse.Status
	apiResponse.StatusCode = apiResponse.RawHttpResponse.StatusCode
	apiResponse.IsSuccessful = (apiResponse.StatusCode == 200) || (apiResponse.StatusCode == 202) || (apiResponse.StatusCode == 404)
	apiResponse.EmailVerificationResponse = &emailResponse
}

func min(x, y int) int {
	if x < y {
		return x
	}
	return y
}
