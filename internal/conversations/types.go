package conversations

import "time"

const (
	SourceGeminiSpark = "gemini-spark"
	SparkDir          = "conversations/spark/"
	SharePrefix       = "/share/c/"
)

// Message is one turn in a saved conversation.
type Message struct {
	Role      string     `json:"role"`
	Text      string     `json:"text"`
	Content   string     `json:"content,omitempty"` // alias accepted from Spark/MCP
	Timestamp *time.Time `json:"timestamp,omitempty"`
}

// Share describes a minted public or unlisted link.
type Share struct {
	Token   string    `json:"token"`
	Public  bool      `json:"public"`
	URL     string    `json:"url"`
	Created time.Time `json:"created"`
}

// Conversation is stored as JSON-LD (+ Turtle sidecar) in the owner's pod.
type Conversation struct {
	Context   any       `json:"@context,omitempty"`
	Type      string    `json:"@type,omitempty"`
	JSONLDID  string    `json:"@id,omitempty"`
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Name      string    `json:"name,omitempty"` // schema:name alias for JSON-LD
	Source    string    `json:"source"`
	SourceURL string    `json:"sourceUrl,omitempty"`
	Created   time.Time `json:"created"`
	Updated   time.Time `json:"updated"`
	DateCreated  string `json:"dateCreated,omitempty"`
	DateModified string `json:"dateModified,omitempty"`
	Messages  []Message `json:"messages"`
	Owner     string    `json:"owner"`
	Creator   string    `json:"creator,omitempty"`
	PodPath   string    `json:"podPath"`
	Resource  string    `json:"resource"`
	MetaTTL   string    `json:"metaTtl,omitempty"`
	Shared    *Share    `json:"share,omitempty"`
}

// SaveResult is what MCP / HTTP return so Spark can confirm the upload.
type SaveResult struct {
	OK             bool          `json:"ok"`
	ID             string        `json:"id"`
	Title          string        `json:"title"`
	ResourceURL    string        `json:"resourceUrl"`
	MetaTTLURL     string        `json:"metaTtlUrl"`
	WebID          string        `json:"webId"`
	Pod            string        `json:"pod"`
	Source         string        `json:"source"`
	Created        time.Time     `json:"created"`
	Modified       time.Time     `json:"modified"`
	MessageCount   int           `json:"messageCount"`
	ShareURL       string        `json:"shareUrl,omitempty"`
	Confirmation   string        `json:"confirmation"`
	Conversation   *Conversation `json:"conversation,omitempty"`
}

// SaveInput is the payload accepted by HTTP, MCP, and paste import.
type SaveInput struct {
	ID        string    `json:"id,omitempty"`
	Title     string    `json:"title,omitempty"`
	Messages  []Message `json:"messages,omitempty"`
	Source    string    `json:"source,omitempty"`
	SourceURL string    `json:"source_url,omitempty"`
	Text      string    `json:"text,omitempty"`
}

type shareRecord struct {
	Token      string    `json:"token"`
	ConvoID    string    `json:"convoId"`
	OwnerWebID string    `json:"ownerWebId"`
	PodPath    string    `json:"podPath"`
	Resource   string    `json:"resource"`
	Public     bool      `json:"public"`
	Created    time.Time `json:"created"`
}

type shareIndexFile struct {
	Shares []shareRecord `json:"shares"`
}

type actor struct {
	WebID   string
	PodPath string
	Handle  string
}
