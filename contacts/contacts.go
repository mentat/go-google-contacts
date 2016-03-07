package contacts

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const basePath = "www.google.com/m8/feeds/contacts/default/full/"

type Client struct {
	AuthManager  AuthManager
	HTTPClient   *http.Client
	DisableHTTPS bool
}

type Feed struct {
	TotalResults int     `xml:"totalResults"`
	StartIndex   int     `xml:"startIndex"`
	ItemsPerPage int     `xml:"itemsPerPage"`
	Entries      []Entry `xml:"entry"`
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
	Primary          string `xml:"primary,attr,omitempty"`
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
	Deleted string `xml:"deleted,attr"`
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

func NewClient(authManager AuthManager) *Client {
	return &Client{
		AuthManager: authManager,
		HTTPClient:  http.DefaultClient,
	}
}

func (c *Client) FetchFeed() (*Feed, error) {
	data, err := c.FetchFeedRaw()
	if err != nil {
		return nil, err
	}

	return unmarshalResponse(data)
}

func (c *Client) FetchFeedRaw() ([]byte, error) {
	accessToken, err := c.AuthManager.AccessToken()

	if err != nil {
		return nil, err
	}

	data, err := c.retrieveFeed(accessToken)
	if err != nil {
		accessToken, err = c.AuthManager.Renew()
		if err != nil {
			return nil, err
		}
		data, err = c.retrieveFeed(accessToken)
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
	protocol := "https"
	if c.DisableHTTPS {
		protocol = "http"
	}
	values := url.Values{}
	values.Set("access_token", accessToken)
	fullUrl := protocol + "://" + basePath + contactID + "?" + values.Encode()
	return c.get(fullUrl)
}

func (c *Client) Save(entry *Entry) (*Entry, error) {
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

func (c *Client) saveContact(accessToken string, entry *Entry) (*Entry, error) {
	protocol := "https"
	if c.DisableHTTPS {
		protocol = "http"
	}
	xmlBytes, err := xml.MarshalIndent(entry, "", "  ")
	if err != nil {
		return nil, err
	}
	values := url.Values{}
	values.Set("access_token", accessToken)

	// TODO: remove this ugly hack when xml namespaces in golang have better support
	//       see e.g. https://github.com/golang/go/issues/12624
	xmlString := fixXml(string(xmlBytes))
	reader := strings.NewReader(xmlString)
	url := protocol + "://" + basePath + entry.GetId() + "?" + values.Encode()

	request, err := http.NewRequest("PUT", url, reader)
	request.Header.Add("GData-Version", "3.0")
	request.Header.Add("Content-Type", "application/atom+xml")
	request.Header.Add("If-Match", entry.ETag)
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
		return nil, errors.New("couldn't save entry; got " + resp.Status + "\nResponse:\n" + string(data))
	}
	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(resp.Body)
	if err != nil {
		return nil, err
	}

	return unmarshalEntry(buf.Bytes())
}

func (c *Client) retrieveFeed(accessToken string) ([]byte, error) {
	protocol := "https"
	if c.DisableHTTPS {
		protocol = "http"
	}
	fullURL := protocol + "://" + basePath

	values := url.Values{}
	// TODO: support pagination
	values.Set("max-results", "10000")
	values.Set("access_token", accessToken)
	return c.get(fullURL + "?" + values.Encode())
}

func (c *Client) get(url string) ([]byte, error) {
	req, err := http.NewRequest("", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("GData-Version", "3.0")
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, errors.New("couldn't fetch given URL; got " + resp.Status)
	}
	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(resp.Body)

	return buf.Bytes(), err
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
