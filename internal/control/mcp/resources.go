package mcp

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/hilather/go-lab-netconf/internal/app"
	"github.com/hilather/go-lab-netconf/internal/capabilities"
	"github.com/hilather/go-lab-netconf/internal/domainerr"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func (s *Server) registerResources() {
	h := s.readResource
	for _, uri := range capabilities.Resources() {
		cap, _ := capabilities.LookupResource(uri)
		name := resourceName(uri)
		desc := cap.Description
		mime := "application/json"
		if strings.Contains(uri, "schema") {
			mime = "application/schema+json"
		}
		if strings.Contains(uri, "{") {
			s.sdk.AddResourceTemplate(&sdk.ResourceTemplate{
				URITemplate: uri, Name: name, Description: desc, MIMEType: mime,
			}, h)
			continue
		}
		s.sdk.AddResource(&sdk.Resource{
			URI: uri, Name: name, Description: desc, MIMEType: mime,
		}, h)
	}
}

func resourceName(uri string) string {
	uri = strings.TrimPrefix(uri, "labnetconf://")
	uri = strings.ReplaceAll(uri, "{", "")
	uri = strings.ReplaceAll(uri, "}", "")
	uri = strings.ReplaceAll(uri, "/", "-")
	if uri == "" {
		return "resource"
	}
	return uri
}

func (s *Server) readResource(ctx context.Context, req *sdk.ReadResourceRequest) (*sdk.ReadResourceResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, rpcError(domainerr.WaitTimeout("request canceled"))
	}
	actor := s.actorFrom(ctx)
	uri := ""
	if req != nil && req.Params != nil {
		uri = req.Params.URI
	}
	if err := s.authorizeResource(actor, uri); err != nil {
		return nil, rpcError(err)
	}
	body, mime, err := s.resourceBody(ctx, actor, uri)
	if err != nil {
		return nil, rpcError(err)
	}
	return &sdk.ReadResourceResult{
		Contents: []*sdk.ResourceContents{{
			URI:      uri,
			MIMEType: mime,
			Text:     string(body),
		}},
	}, nil
}

func (s *Server) resourceBody(ctx context.Context, actor app.Actor, uri string) ([]byte, string, error) {
	_ = actor
	switch {
	case uri == "labnetconf://state":
		v, err := s.svc.State(ctx)
		if err != nil {
			return nil, "", err
		}
		view, err := fromStateView(v)
		if err != nil {
			return nil, "", err
		}
		b, err := marshalAPI(view)
		return b, "application/json", err
	case uri == "labnetconf://schema/config":
		raw, err := s.svc.Schema(ctx)
		if err != nil {
			return nil, "", err
		}
		if len(raw) == 0 {
			raw = json.RawMessage("{}")
		}
		return raw, "application/schema+json", nil
	case strings.HasPrefix(uri, "labnetconf://profiles/"):
		name := strings.TrimPrefix(uri, "labnetconf://profiles/")
		if name == "" || strings.Contains(name, "/") {
			return nil, "", domainerr.NotFound("not found")
		}
		p, err := s.svc.GetProfile(ctx, name)
		if err != nil {
			return nil, "", err
		}
		b, err := marshalAPI(fromProfile(p))
		return b, "application/json", err
	case strings.HasPrefix(uri, "labnetconf://datastores/"):
		rest := strings.TrimPrefix(uri, "labnetconf://datastores/")
		profile, store, ok := strings.Cut(rest, "/")
		if !ok || profile == "" || store == "" || strings.Contains(store, "/") {
			return nil, "", domainerr.NotFound("not found")
		}
		raw, err := s.svc.GetDatastore(ctx, profile, store)
		if err != nil {
			return nil, "", err
		}
		return raw, "application/json", nil
	case strings.HasPrefix(uri, "labnetconf://notifications/"):
		id := strings.TrimPrefix(uri, "labnetconf://notifications/")
		if id == "" || strings.Contains(id, "/") {
			return nil, "", domainerr.NotFound("not found")
		}
		n, err := s.svc.GetNotification(ctx, id)
		if err != nil {
			return nil, "", err
		}
		b, err := marshalAPI(fromNotification(n))
		return b, "application/json", err
	default:
		return nil, "", domainerr.NotFound("not found")
	}
}
