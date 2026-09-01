package conversations

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"solid-go/internal/authn"
	"solid-go/internal/identityapi"
	"solid-go/internal/rdf"
	"solid-go/internal/resourcestore"
	"solid-go/internal/wac"
)

var (
	ErrUnauthorized = errors.New("unauthorized")
	ErrNotFound     = errors.New("conversation not found")
	ErrForbidden    = errors.New("forbidden")
)

// AuditHook is called after each mutating Solid write.
type AuditHook func(ctx context.Context, agentWebID, method, path string, body []byte)

// Service stores Spark conversations in the signed-in human's Solid pod.
type Service struct {
	Store   *resourcestore.Store
	Tokens  *authn.TokenService
	IDP     *identityapi.Service
	BaseURL string
	OnAudit AuditHook
	HTTP    *http.Client

	mu     sync.RWMutex
	shares map[string]shareRecord // token or convo id (if public) -> record
}

func New(store *resourcestore.Store, tokens *authn.TokenService, idp *identityapi.Service, baseURL string) *Service {
	s := &Service{
		Store:   store,
		Tokens:  tokens,
		IDP:     idp,
		BaseURL: strings.TrimRight(baseURL, "/"),
		HTTP:    defaultHTTP(),
		shares:  map[string]shareRecord{},
	}
	s.loadShares()
	return s
}

func defaultHTTP() *http.Client {
	return &http.Client{Timeout: 12 * time.Second}
}

func (s *Service) jsonLDContext() any {
	return map[string]any{
		"@vocab": "https://schema.org/",
		"schema": "https://schema.org/",
		"openid": "https://www.w3.org/ns/solid/terms#",
	}
}

func (s *Service) resourceURL(p string) string {
	p = strings.TrimPrefix(p, "/")
	if p == "" {
		return s.BaseURL + "/"
	}
	return s.BaseURL + "/" + p
}

func (s *Service) sparkDir(podPath string) string {
	return strings.TrimSuffix(podPath, "/") + "/" + SparkDir
}

func (s *Service) convoPath(podPath, id string) string {
	return s.sparkDir(podPath) + id + ".json"
}

func (s *Service) metaPath(podPath, id string) string {
	return s.sparkDir(podPath) + id + ".ttl"
}

func (s *Service) shareMetaPath(podPath, id string) string {
	return s.sparkDir(podPath) + id + ".share.json"
}

func (s *Service) fire(ctx context.Context, agent, method, path string, body []byte) {
	if s.OnAudit != nil {
		s.OnAudit(ctx, agent, method, path, body)
	}
}

func (s *Service) putAudited(ctx context.Context, actor *actor, path, ct string, body []byte) error {
	if _, err := s.Store.Put(ctx, path, ct, body, "", ""); err != nil {
		return err
	}
	s.fire(ctx, actor.WebID, "PUT", path, body)
	return nil
}

func (s *Service) deleteAudited(ctx context.Context, actor *actor, path string) error {
	if err := s.Store.Delete(ctx, path, ""); err != nil && !errors.Is(err, resourcestore.ErrNotFound) {
		return err
	}
	s.fire(ctx, actor.WebID, "DELETE", path, nil)
	return nil
}

func (s *Service) ensurePrivateContainer(ctx context.Context, actor *actor) error {
	dir := s.sparkDir(actor.PodPath)
	if err := s.Store.EnsureContainer(ctx, dir); err != nil {
		return err
	}
	parent := strings.TrimSuffix(actor.PodPath, "/") + "/conversations/"
	acl := wac.OwnerOnlyACL(s.resourceURL(parent), actor.WebID)
	if err := s.putAudited(ctx, actor, strings.TrimSuffix(parent, "/")+"/.acl", "text/turtle", []byte(acl)); err != nil {
		return err
	}
	sparkACL := wac.OwnerOnlyACL(s.resourceURL(dir), actor.WebID)
	return s.putAudited(ctx, actor, strings.TrimSuffix(dir, "/")+"/.acl", "text/turtle", []byte(sparkACL))
}

func (s *Service) writeOwnerACL(ctx context.Context, actor *actor, resource string) error {
	acl := wac.OwnerOnlyACL(s.resourceURL(resource), actor.WebID)
	return s.putAudited(ctx, actor, resourcestore.AuxPath(resource, ".acl"), "text/turtle", []byte(acl))
}

func (s *Service) writePublicACL(ctx context.Context, actor *actor, resource string) error {
	acl := wac.PublicReadACL(s.resourceURL(resource), actor.WebID)
	return s.putAudited(ctx, actor, resourcestore.AuxPath(resource, ".acl"), "text/turtle", []byte(acl))
}

func (s *Service) metadataTurtle(c *Conversation) string {
	g := rdf.NewGraph()
	prefixes := map[string]string{
		"schema": "https://schema.org/",
		"dct":    "http://purl.org/dc/terms/",
		"solid":  "http://www.w3.org/ns/solid/terms#",
	}
	id := s.resourceURL(c.Resource)
	g.AddIRI(id, "http://www.w3.org/1999/02/22-rdf-syntax-ns#type", "https://schema.org/Conversation")
	g.AddLiteral(id, "https://schema.org/name", c.Title)
	g.AddLiteral(id, "https://schema.org/identifier", c.ID)
	g.AddLiteral(id, "http://purl.org/dc/terms/created", c.Created.UTC().Format(time.RFC3339))
	g.AddLiteral(id, "http://purl.org/dc/terms/modified", c.Updated.UTC().Format(time.RFC3339))
	g.AddLiteral(id, "https://schema.org/creator", c.Owner)
	g.AddLiteral(id, "http://purl.org/dc/terms/source", firstNonEmpty(c.Source, SourceGeminiSpark))
	if c.SourceURL != "" {
		g.AddIRI(id, "https://schema.org/url", c.SourceURL)
	}
	g.AddIRI(id, "https://schema.org/encoding", s.resourceURL(c.Resource))
	return rdf.SerializeTurtle(g, prefixes)
}

func (s *Service) decorate(c *Conversation) {
	c.Context = s.jsonLDContext()
	c.Type = "Conversation"
	c.JSONLDID = s.resourceURL(c.Resource)
	c.MetaTTL = s.resourceURL(s.metaPath(c.PodPath, c.ID))
	if rec, ok := s.lookupShare(c.ID); ok {
		c.Shared = &Share{
			Token:   rec.Token,
			Public:  rec.Public,
			URL:     s.shareURL(rec),
			Created: rec.Created,
		}
	}
}

func (s *Service) shareURL(rec shareRecord) string {
	id := rec.Token
	if rec.Public {
		id = rec.ConvoID
	}
	return s.BaseURL + SharePrefix + id
}

func (s *Service) Save(ctx context.Context, actor *actor, in SaveInput) (*Conversation, error) {
	if actor == nil || actor.WebID == "" || actor.PodPath == "" {
		return nil, ErrUnauthorized
	}
	var parsed *Conversation
	var err error
	text := strings.TrimSpace(in.Text)
	if IsGeminiShareURL(text) && in.SourceURL == "" {
		in.SourceURL = normalizeShareURL(text)
		text = ""
	}
	if IsGeminiShareURL(in.SourceURL) && len(in.Messages) == 0 && text == "" {
		parsed, err = s.ImportShareURL(ctx, in.SourceURL)
		if err != nil {
			return nil, err
		}
	} else if len(in.Messages) > 0 {
		parsed = &Conversation{
			Title:     in.Title,
			Source:    firstNonEmpty(in.Source, SourceGeminiSpark),
			SourceURL: in.SourceURL,
			Messages:  normalizeMessages(in.Messages),
		}
		if parsed.Title == "" {
			parsed.Title = titleFromMessages(parsed.Messages)
		}
	} else if text != "" {
		parsed, err = ParseTranscript(text)
		if err != nil {
			return nil, err
		}
		if in.Title != "" {
			parsed.Title = in.Title
		}
		if in.SourceURL != "" {
			parsed.SourceURL = in.SourceURL
		}
	} else {
		return nil, fmt.Errorf("messages or text required")
	}
	if len(parsed.Messages) == 0 {
		return nil, fmt.Errorf("messages required")
	}

	id := strings.TrimSpace(in.ID)
	if id == "" {
		id = uuid.NewString()
	}
	now := time.Now().UTC()
	created := now
	resource := s.convoPath(actor.PodPath, id)
	if existing, err := s.load(ctx, actor, id); err == nil && existing != nil {
		created = existing.Created
	}

	c := &Conversation{
		ID:        id,
		Title:     firstNonEmpty(in.Title, parsed.Title, titleFromMessages(parsed.Messages)),
		Source:    firstNonEmpty(in.Source, parsed.Source, SourceGeminiSpark),
		SourceURL: firstNonEmpty(in.SourceURL, parsed.SourceURL),
		Created:   created,
		Updated:   now,
		Messages:  parsed.Messages,
		Owner:     actor.WebID,
		PodPath:   actor.PodPath,
		Resource:  resource,
	}
	if err := s.ensurePrivateContainer(ctx, actor); err != nil {
		return nil, err
	}
	s.decorate(c)
	body, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := s.putAudited(ctx, actor, resource, "application/ld+json", body); err != nil {
		return nil, err
	}
	ttl := []byte(s.metadataTurtle(c))
	if err := s.putAudited(ctx, actor, s.metaPath(actor.PodPath, id), "text/turtle", ttl); err != nil {
		return nil, err
	}
	if err := s.writeOwnerACL(ctx, actor, resource); err != nil {
		return nil, err
	}
	s.decorate(c)
	return c, nil
}

func (s *Service) load(ctx context.Context, actor *actor, id string) (*Conversation, error) {
	res, err := s.Store.Get(ctx, s.convoPath(actor.PodPath, id))
	if err != nil {
		if errors.Is(err, resourcestore.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	var c Conversation
	if err := json.Unmarshal(res.Body, &c); err != nil {
		return nil, err
	}
	c.Resource = s.convoPath(actor.PodPath, id)
	c.PodPath = actor.PodPath
	c.Owner = actor.WebID
	s.decorate(&c)
	return &c, nil
}

func (s *Service) Get(ctx context.Context, actor *actor, id string) (*Conversation, error) {
	if actor == nil {
		return nil, ErrUnauthorized
	}
	return s.load(ctx, actor, id)
}

func (s *Service) List(ctx context.Context, actor *actor) ([]*Conversation, error) {
	if actor == nil {
		return nil, ErrUnauthorized
	}
	dir := s.sparkDir(actor.PodPath)
	children, err := s.Store.List(ctx, dir)
	if err != nil {
		if errors.Is(err, resourcestore.ErrNotFound) {
			return []*Conversation{}, nil
		}
		return nil, err
	}
	var out []*Conversation
	for _, child := range children {
		base := path.Base(child)
		if !strings.HasSuffix(base, ".json") || strings.HasSuffix(base, ".share.json") {
			continue
		}
		id := strings.TrimSuffix(base, ".json")
		c, err := s.load(ctx, actor, id)
		if err != nil {
			continue
		}
		out = append(out, c)
	}
	return out, nil
}

func (s *Service) Share(ctx context.Context, actor *actor, id string, public bool) (*Conversation, error) {
	c, err := s.load(ctx, actor, id)
	if err != nil {
		return nil, err
	}
	if rec, ok := s.lookupShare(id); ok {
		rec.Public = public
		if err := s.persistShare(ctx, actor, rec); err != nil {
			return nil, err
		}
		if public {
			if err := s.writePublicACL(ctx, actor, rec.Resource); err != nil {
				return nil, err
			}
		} else {
			if err := s.writeOwnerACL(ctx, actor, rec.Resource); err != nil {
				return nil, err
			}
		}
		s.decorate(c)
		return c, nil
	}
	token, err := randomToken()
	if err != nil {
		return nil, err
	}
	rec := shareRecord{
		Token:      token,
		ConvoID:    id,
		OwnerWebID: actor.WebID,
		PodPath:    actor.PodPath,
		Resource:   c.Resource,
		Public:     public,
		Created:    time.Now().UTC(),
	}
	if err := s.persistShare(ctx, actor, rec); err != nil {
		return nil, err
	}
	if public {
		if err := s.writePublicACL(ctx, actor, c.Resource); err != nil {
			return nil, err
		}
	} else {
		if err := s.writeOwnerACL(ctx, actor, c.Resource); err != nil {
			return nil, err
		}
	}
	s.decorate(c)
	return c, nil
}

func (s *Service) Unshare(ctx context.Context, actor *actor, id string) error {
	c, err := s.load(ctx, actor, id)
	if err != nil {
		return err
	}
	if err := s.writeOwnerACL(ctx, actor, c.Resource); err != nil {
		return err
	}
	_ = s.deleteAudited(ctx, actor, s.shareMetaPath(actor.PodPath, id))
	s.dropShare(id)
	return nil
}

func (s *Service) persistShare(ctx context.Context, actor *actor, rec shareRecord) error {
	body, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	if err := s.putAudited(ctx, actor, s.shareMetaPath(actor.PodPath, rec.ConvoID), "application/json", body); err != nil {
		return err
	}
	s.mu.Lock()
	s.shares[rec.Token] = rec
	s.shares[rec.ConvoID] = rec
	s.mu.Unlock()
	return s.saveShares()
}

func (s *Service) dropShare(convoID string) {
	s.mu.Lock()
	if rec, ok := s.shares[convoID]; ok {
		delete(s.shares, rec.Token)
		delete(s.shares, rec.ConvoID)
	}
	for k, rec := range s.shares {
		if rec.ConvoID == convoID {
			delete(s.shares, k)
		}
	}
	s.mu.Unlock()
	_ = s.saveShares()
}

func (s *Service) lookupShare(id string) (shareRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rec, ok := s.shares[id]
	return rec, ok
}

func (s *Service) PublicGet(ctx context.Context, id string) (*Conversation, error) {
	rec, ok := s.lookupShare(id)
	if !ok {
		return nil, ErrNotFound
	}
	if !rec.Public && id != rec.Token {
		return nil, ErrNotFound
	}
	res, err := s.Store.Get(ctx, rec.Resource)
	if err != nil {
		if errors.Is(err, resourcestore.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	var c Conversation
	if err := json.Unmarshal(res.Body, &c); err != nil {
		return nil, err
	}
	c.Resource = rec.Resource
	c.PodPath = rec.PodPath
	c.Owner = rec.OwnerWebID
	s.decorate(&c)
	return &c, nil
}

func (s *Service) loadShares() {
	res, err := s.Store.Get(context.Background(), ".openid/shares.json")
	if err != nil {
		return
	}
	var file shareIndexFile
	if json.Unmarshal(res.Body, &file) != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, rec := range file.Shares {
		s.shares[rec.Token] = rec
		s.shares[rec.ConvoID] = rec
	}
}

func (s *Service) saveShares() error {
	s.mu.RLock()
	seen := map[string]shareRecord{}
	for _, rec := range s.shares {
		seen[rec.Token] = rec
	}
	s.mu.RUnlock()
	file := shareIndexFile{}
	for _, rec := range seen {
		file.Shares = append(file.Shares, rec)
	}
	body, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	_, err = s.Store.Put(context.Background(), ".openid/shares.json", "application/json", body, "", "")
	return err
}

func randomToken() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

func podFromWebID(webID, baseURL string) (podPath, handle string) {
	webID = strings.TrimSpace(webID)
	if i := strings.Index(webID, "#"); i >= 0 {
		webID = webID[:i]
	}
	baseURL = strings.TrimRight(baseURL, "/")
	rest := webID
	if strings.HasPrefix(webID, baseURL+"/") {
		rest = strings.TrimPrefix(webID, baseURL+"/")
	} else if u := webID; strings.Contains(u, "://") {
		without := strings.SplitN(u, "://", 2)[1]
		if i := strings.Index(without, "/"); i >= 0 {
			rest = without[i+1:]
		}
	}
	rest = strings.TrimPrefix(rest, "/")
	parts := strings.Split(rest, "/")
	if len(parts) == 0 || parts[0] == "" {
		return "", ""
	}
	return parts[0] + "/", parts[0]
}

func (s *Service) actorFromRequest(r *http.Request) (*actor, error) {
	if s.IDP != nil {
		if acc := s.IDP.AccountFromRequest(r); acc != nil {
			return &actor{WebID: acc.WebID, PodPath: acc.PodPath, Handle: acc.Handle}, nil
		}
	}
	if s.Tokens == nil {
		return nil, ErrUnauthorized
	}
	creds, err := s.Tokens.Extract(r)
	if err != nil || creds == nil || creds.WebID == "" {
		return nil, ErrUnauthorized
	}
	if s.IDP != nil {
		if acc := s.IDP.FindByWebID(creds.WebID); acc != nil {
			return &actor{WebID: acc.WebID, PodPath: acc.PodPath, Handle: acc.Handle}, nil
		}
	}
	pod, handle := podFromWebID(creds.WebID, s.BaseURL)
	if pod == "" {
		return nil, ErrUnauthorized
	}
	return &actor{WebID: creds.WebID, PodPath: pod, Handle: handle}, nil
}

func (s *Service) actorFromToken(token string) (*actor, error) {
	if token == "" || s.Tokens == nil {
		return nil, ErrUnauthorized
	}
	creds, err := s.Tokens.Parse(token)
	if err != nil || creds == nil || creds.WebID == "" {
		return nil, ErrUnauthorized
	}
	if s.IDP != nil {
		if acc := s.IDP.FindByWebID(creds.WebID); acc != nil {
			return &actor{WebID: acc.WebID, PodPath: acc.PodPath, Handle: acc.Handle}, nil
		}
	}
	pod, handle := podFromWebID(creds.WebID, s.BaseURL)
	if pod == "" {
		return nil, ErrUnauthorized
	}
	return &actor{WebID: creds.WebID, PodPath: pod, Handle: handle}, nil
}
