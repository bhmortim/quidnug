// Package core. handlers_gossip.go — gossip and node-domain HTTP handlers.
package core

import "net/http"

// GetNodeDomainsHandler returns this node's supported domains for discovery
func (node *QuidnugNode) GetNodeDomainsHandler(w http.ResponseWriter, r *http.Request) {
	WriteSuccess(w, map[string]interface{}{
		"nodeId":  node.NodeID,
		"domains": node.SupportedDomains,
	})
}

// UpdateNodeDomainsHandler updates this node's supported domains
func (node *QuidnugNode) UpdateNodeDomainsHandler(w http.ResponseWriter, r *http.Request) {
	if !node.AllowDomainRegistration {
		WriteError(w, http.StatusForbidden, "FORBIDDEN", "Domain registration is not allowed on this node")
		return
	}

	var req struct {
		Domains []string `json:"domains"`
	}
	if err := DecodeJSONBody(w, r, &req); err != nil {
		return
	}

	// Validate domain patterns
	for _, domain := range req.Domains {
		if domain == "" {
			WriteFieldError(w, "INVALID_DOMAIN", "Domain cannot be empty", []string{"domains"})
			return
		}
		if !ValidateStringField(domain, MaxDomainLength) {
			WriteFieldError(w, "INVALID_DOMAIN", "Domain too long or contains control characters", []string{"domains"})
			return
		}
	}

	node.SupportedDomains = req.Domains

	// Trigger gossip broadcast when domains change
	go node.BroadcastDomainInfo()

	WriteSuccess(w, map[string]interface{}{
		"nodeId":  node.NodeID,
		"domains": node.SupportedDomains,
	})
}

// ReceiveDomainGossipHandler handles incoming domain gossip messages
func (node *QuidnugNode) ReceiveDomainGossipHandler(w http.ResponseWriter, r *http.Request) {
	var gossip DomainGossip
	if err := DecodeJSONBody(w, r, &gossip); err != nil {
		return
	}

	if err := node.ReceiveDomainGossip(gossip); err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_GOSSIP", err.Error())
		return
	}

	WriteSuccess(w, map[string]interface{}{
		"status": "accepted",
		"nodeId": node.NodeID,
	})
}

// QueryTitleRegistryHandler handles title registry queries
