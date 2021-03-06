package contacts

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const groupPath = "www.google.com/m8/feeds/groups/default/full/"
const basePath = "www.google.com/m8/feeds/contacts/default/full/"

type Client struct {
	AuthManager  AuthManager
	HTTPClient   *http.Client
	DisableHTTPS bool
	Cancel       context.CancelFunc
}

type Feed struct {
	TotalResults int     `xml:"totalResults"`
	StartIndex   int     `xml:"startIndex"`
	ItemsPerPage int     `xml:"itemsPerPage"`
	Entries      []Entry `xml:"entry"`
}

type EntryType interface {
	GetURI() string
	GetEtag() string
}

type Entry struct {
	XMLName xml.Name  `xml:"entry"`
	ETag    string    `xml:"etag,attr"`
	Id      string    `xml:"id"`
	Updated time.Time `xml:"updated"`
	Title   string    `xml:"title"`
	Content string    `xml:"content"`
	Links   []Link    `xml:"link"`

	// gd namespace
	Name                      Name                      `xml:"name,omitempty"`
	InstantMessengers         []InstantMessenger        `xml:"im"`
	Organization              Organization              `xml:"organization,omitempty"`
	Emails                    []Email                   `xml:"email"`
	PhoneNumbers              []PhoneNumber             `xml:"phoneNumber"`
	StructuredPostalAddresses []StructuredPostalAddress `xml:"structuredPostalAddress"`
	ExtendedProperties        []ExtendedProperty        `xml:"extendedProperty"`

	// gContact namespace
	Birthday            Birthday              `xml:"birthday,omitempty"`
	Nickname            string                `xml:"nickname"`
	FileAs              string                `xml:"fileAs"`
	Events              []Event               `xml:"event"`
	Relations           []Relation            `xml:"relation"`
	UserDefinedFields   []UserDefinedField    `xml:"userDefinedField"`
	Websites            []Website             `xml:"website"`
	GroupMembershipInfo []GroupMembershipInfo `xml:"groupMembershipInfo"`
}

func (e *Entry) GetURI() string {
	return e.Id
}

func (e *Entry) GetEtag() string {
	return e.ETag
}

func (e *Entry) GetId() string {
	pos := strings.LastIndex(e.Id, "/")
	return e.Id[pos+1 : len(e.Id)]
}

type Name struct {
	FullName       string     `xml:"fullName,omitempty"`
	NamePrefix     string     `xml:"namePrefix,omitempty"`
	GivenName      GivenName  `xml:"givenName,omitempty"`
	AdditionalName string     `xml:"additionalName,omitempty"`
	FamilyName     FamilyName `xml:"familyName,omitempty"`
	NameSuffix     string     `xml:"nameSuffix,omitempty"`
}

type GivenName struct {
	Phonetic string `xml:"yomi,attr,omitempty"`
	Value    string `xml:",chardata"`
}

type FamilyName struct {
	Phonetic string `xml:"yomi,attr,omitempty"`
	Value    string `xml:",chardata"`
}

type Email struct {
	Address string `xml:"address,attr"`
	Primary bool   `xml:"primary,attr,omitempty"`
	Label   string `xml:"label,attr,omitempty"`
	Rel     string `xml:"rel,attr,omitempty"`
}

type Event struct {
	When  When   `xml:"when"`
	Label string `xml:"label,attr,omitempty"`
	Rel   string `xml:"rel,attr,omitempty"`
}

type Birthday struct {
	When string `xml:"when,attr,omitempty"`
}

type When struct {
	StartTime string `xml:"startTime,attr"`
}

type Relation struct {
	Rel   string `xml:"rel,attr"`
	Value string `xml:",chardata"`
}

type UserDefinedField struct {
	Key   string `xml:"key,attr"`
	Value string `xml:"value,attr"`
}

type Website struct {
	Rel   string `xml:"rel,attr,omitempty"`
	Label string `xml:"label,attr,omitempty"`
	Href  string `xml:"href,attr"`
}

type PhoneNumber struct {
	Label string `xml:"label,attr,omitempty"`
	Rel   string `xml:"rel,attr,omitempty"`
	Uri   string `xml:"uri,attr,omitempty"`
	Value string `xml:",chardata"`
}

type PostalAddress struct {
	Rel     string `xml:"rel,attr"`
	Primary string `xml:"primary,attr"`
	Label   string `xml:"label,attr"`
	Value   string `xml:",chardata"`
}

type StructuredPostalAddress struct {
	Rel              string `xml:"rel,attr,omitempty"`
	Primary          bool   `xml:"primary,attr,omitempty"`
	Label            string `xml:"label,attr,omitempty"`
	City             string `xml:"city,omitempty"`
	Street           string `xml:"street,omitempty"`
	Region           string `xml:"region,omitempty"`
	Postcode         string `xml:"postcode,omitempty"`
	Country          string `xml:"country,omitempty"`
	FormattedAddress string `xml:"formattedAddress,omitempty"`
}

type InstantMessenger struct {
	Address  string `xml:"address,attr"`
	Protocol string `xml:"protocol,attr"`
	Rel      string `xml:"rel,attr"`
}

type ExtendedProperty struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}

type GroupMembershipInfo struct {
	Deleted bool   `xml:"deleted,attr"`
	Href    string `xml:"href,attr"`
}

type Link struct {
	Rel  string `xml:"rel,attr,omitempty"`
	Type string `xml:"type,attr"`
	Href string `xml:"href,attr"`
}

type Organization struct {
	Rel      string `xml:"rel,attr,omitempty"`
	OrgName  string `xml:"orgName,omitempty"`
	OrgTitle string `xml:"orgTitle,omitempty"`
}

type ContactQuery struct {
	Query      string
	MaxResults int64
	StartIndex int64
	Group      string
}

// NewQuery - return a new ContactQuery struct initialized
func NewQuery() *ContactQuery {
	return &ContactQuery{
		MaxResults: 100,
		StartIndex: 1,
	}
}

func NewClient(authManager AuthManager) *Client {
	return &Client{
		AuthManager: authManager,
		HTTPClient:  http.DefaultClient,
	}
}

// FetchContactImage - fetch a contact's image whos URL is 'href'
// returns 3-tuple of picture (bytes, mimetype, error)
func (c *Client) FetchContactImage(href string) ([]byte, string, error) {

	accessToken, err := c.AuthManager.AccessToken()

	if err != nil {
		return nil, "", err
	}

	return c.get(href, accessToken)
}

func (c *Client) FetchGroups(start, max int64, query string) (*Feed, error) {
	data, err := c.FetchGroupsRaw(start, max, query)
	if err != nil {
		return nil, err
	}

	return unmarshalResponse(data)
}

func (c *Client) FetchGroupsRaw(start, max int64, query string) ([]byte, error) {
	accessToken, err := c.AuthManager.AccessToken()

	if err != nil {
		return nil, err
	}

	data, err := c.retrieveGroups(start, max, query, accessToken)
	if err != nil {
		accessToken, err = c.AuthManager.Renew()
		if err != nil {
			return nil, err
		}
		data, err = c.retrieveGroups(start, max, query, accessToken)
		if err != nil {
			return nil, err
		}
	}

	return data, nil
}

func (c *Client) FetchFeed(query *ContactQuery) (*Feed, error) {
	data, err := c.FetchFeedRaw(query)
	if err != nil {
		return nil, err
	}

	return unmarshalResponse(data)
}

func (c *Client) FetchFeedRaw(query *ContactQuery) ([]byte, error) {
	accessToken, err := c.AuthManager.AccessToken()

	if err != nil {
		return nil, err
	}

	data, err := c.retrieveFeed(query, accessToken)
	if err != nil {
		accessToken, err = c.AuthManager.Renew()
		if err != nil {
			return nil, err
		}
		data, err = c.retrieveFeed(query, accessToken)
		if err != nil {
			return nil, err
		}
	}

	return data, nil
}

func (c *Client) FetchContact(contactID string) (*Entry, error) {
	data, err := c.FetchContactRaw(contactID)
	if err != nil {
		return nil, err
	}
	return unmarshalEntry(data)
}

func (c *Client) FetchContactRaw(contactID string) ([]byte, error) {
	accessToken, err := c.AuthManager.AccessToken()

	if err != nil {
		return nil, err
	}

	data, err := c.fetchContact(accessToken, contactID)
	if err != nil {
		accessToken, err = c.AuthManager.Renew()
		if err != nil {
			return nil, err
		}
		data, err = c.fetchContact(accessToken, contactID)
		if err != nil {
			return nil, err
		}
	}

	return data, nil
}

func (c *Client) fetchContact(accessToken, contactID string) ([]byte, error) {
	values := url.Values{}
	//values.Set("access_token", accessToken)
	//fullUrl := protocol + "://" + basePath + contactID + "?" + values.Encode()

	contactID = strings.Replace(contactID, "/base/", "/full/", -1)
	bytes, _, err := c.get(contactID+"?"+values.Encode(), accessToken)

	return bytes, err
}

func (c *Client) Save(entry EntryType) (*Entry, error) {
	accessToken, err := c.AuthManager.AccessToken()

	if err != nil {
		return nil, err
	}

	data, err := c.saveContact(accessToken, entry)
	if err != nil {
		accessToken, err = c.AuthManager.Renew()
		if err != nil {
			return nil, err
		}
		data, err = c.saveContact(accessToken, entry)
		if err != nil {
			return nil, err
		}
	}

	return data, nil
}

func (c *Client) SaveRaw(entry EntryType) (*bytes.Buffer, error) {
	accessToken, err := c.AuthManager.AccessToken()

	if err != nil {
		return nil, err
	}

	data, err := c.saveContactRaw(accessToken, entry)
	if err != nil {
		accessToken, err = c.AuthManager.Renew()
		if err != nil {
			return nil, err
		}
		data, err = c.saveContactRaw(accessToken, entry)
		if err != nil {
			return nil, err
		}
	}

	return data, nil
}

func (c *Client) saveContactRaw(accessToken string, entry EntryType) (*bytes.Buffer, error) {

	xmlBytes, err := xml.MarshalIndent(entry, "", "  ")
	if err != nil {
		return nil, err
	}

	// TODO: remove this ugly hack when xml namespaces in golang have better support
	//       see e.g. https://github.com/golang/go/issues/12624
	xmlString := fixXml(string(xmlBytes))
	reader := strings.NewReader(xmlString)
	url := entry.GetURI()

	request, err := http.NewRequest("PUT", url, reader)
	request.Header.Add("GData-Version", "3.0")
	request.Header.Add("Content-Type", "application/atom+xml")
	request.Header.Add("If-Match", entry.GetEtag())

	if accessToken != "" {
		request.Header.Add("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	}

	if err != nil {
		return nil, err
	}

	resp, err := c.HTTPClient.Do(request)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		data, _ := ioutil.ReadAll(resp.Body)
		return nil, errors.New("couldn't save entry: " + url + "; got " + resp.Status + "\nResponse:\n" + string(data))
	}

	buf := new(bytes.Buffer)

	_, err = buf.ReadFrom(resp.Body)
	if err != nil {
		return nil, err
	}

	return buf, nil
}

func (c *Client) saveContact(accessToken string, entry EntryType) (*Entry, error) {

	buf, err := c.saveContactRaw(accessToken, entry)

	if err != nil {
		return nil, err
	}

	return unmarshalEntry(buf.Bytes())
}

func (c *Client) retrieveGroups(start, max int64, query, accessToken string) ([]byte, error) {

	protocol := "https"
	if c.DisableHTTPS {
		protocol = "http"
	}
	fullURL := protocol + "://" + groupPath

	values := url.Values{}
	values.Set("max-results", fmt.Sprintf("%d", max))
	values.Set("start-index", fmt.Sprintf("%d", start))

	if query != "" {
		values.Set("q", query)
	}

	bytes, _, err := c.get(fullURL+"?"+values.Encode(), accessToken)

	return bytes, err

}

func (c *Client) retrieveFeed(query *ContactQuery, accessToken string) ([]byte, error) {
	protocol := "https"
	if c.DisableHTTPS {
		protocol = "http"
	}
	fullURL := protocol + "://" + basePath

	values := url.Values{}
	values.Set("max-results", fmt.Sprintf("%d", query.MaxResults))
	values.Set("start-index", fmt.Sprintf("%d", query.StartIndex))

	if query.Query != "" {
		values.Set("q", query.Query)
	}

	if query.Group != "" {
		values.Set("group", query.Group)
	}

	bytes, _, err := c.get(fullURL+"?"+values.Encode(), accessToken)

	return bytes, err
}

func (c *Client) get(url, accessToken string) ([]byte, string, error) {
	req, err := http.NewRequest("", url, nil)

	if err != nil {
		return nil, "", err
	}

	req.Header.Add("GData-Version", "3.0")
	if accessToken != "" {
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	}

	resp, err := c.HTTPClient.Do(req)

	if err != nil {
		return nil, "", err
	}

	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		bodyText, _ := ioutil.ReadAll(resp.Body)
		return nil, "", fmt.Errorf("couldn't fetch given URL; got %s: %s", resp.Status, bodyText)
	}

	mime := resp.Header.Get("Content-Type")

	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(resp.Body)

	return buf.Bytes(), mime, err
}

func unmarshalResponse(data []byte) (*Feed, error) {
	retrieveResponse := &Feed{}
	if err := xml.Unmarshal(data, retrieveResponse); err != nil {
		return nil, err
	}

	return retrieveResponse, nil
}

func unmarshalEntry(data []byte) (*Entry, error) {
	entry := &Entry{}
	if err := xml.Unmarshal(data, entry); err != nil {
		return nil, err
	}

	return entry, nil
}

// fixXml is an ugly hack to remove empty tags and add namespaces :-(
func fixXml(original string) string {
	rules := []string{}

	// omit empty tags
	rules = append(rules, "<name></name>", "")
	rules = append(rules, "<organization></organization>", "")

	// add `gd` XML namespace
	gdTags := []string{
		"name",
		"fullName",
		"namePrefix",
		"givenName",
		"additionalName",
		"familyName",
		"nameSuffix",

		"extendedProperty",

		"organization",
		"orgName",
		"orgTitle",

		"email",
		"im",
		"phoneNumber",

		"structuredPostalAddress",
		"formattedAddress",
		"street",
		"pobox",
		"neighborhood",
		"city",
		"region",
		"postcode",
		"country",

		"when",
	}
	for _, tag := range gdTags {
		rules = append(rules,
			"<"+tag+" ", "<gd:"+tag+" ",
			"<"+tag+">", "<gd:"+tag+">",
			"</"+tag+">", "</gd:"+tag+">")
	}

	// ad `gContact` XML namespace
	gContactTags := []string{
		"groupMembershipInfo",
		"nickname",
		"birthday",
		"fileAs",
		"event",
		"relation",
		"userDefinedField",
		"website",
		"groupMembershipInfo",
	}
	for _, tag := range gContactTags {
		rules = append(rules,
			"<"+tag+" ", "<gContact:"+tag+" ",
			"<"+tag+">", "<gContact:"+tag+">",
			"</"+tag+">", "</gContact:"+tag+">")
	}

	replacer := strings.NewReplacer(rules...)
	return replacer.Replace(original)
}
