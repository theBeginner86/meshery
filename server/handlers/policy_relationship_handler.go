package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/layer5io/meshery/server/models"
	"github.com/layer5io/meshkit/models/meshmodel/core/v1alpha1"
	"github.com/layer5io/meshkit/models/meshmodel/core/policies"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

// swagger:route POST /api/policies/run_policy GetRegoPolicyForDesignFile idGetRegoPolicyForDesignFile
// Handle POST request for running the set of policies on the design file, the policies are picked from the policies directory and query is sent to find all the relationships around the services in the given design file
//
// responses:
// 200
func (h *Handler) GetRegoPolicyForDesignFile(
	rw http.ResponseWriter,
	r *http.Request,
	_ *models.Preference,
	_ *models.User,
	_ models.Provider,
) {
	defer func() {
		_ = r.Body.Close()
	}()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		logrus.Error(ErrRequestBody(err))
		http.Error(rw, ErrRequestBody(err).Error(), http.StatusBadRequest)

		rw.WriteHeader((http.StatusBadRequest))
		return
	}

	name := mux.Vars(r)["name"]
	typ := mux.Vars(r)["model"]
	limitstr := r.URL.Query().Get("pagesize")
	var limit int
	if limitstr != "all" {
		limit, _ = strconv.Atoi(limitstr)
		if limit == 0 { //If limit is unspecified then it defaults to 25
			limit = DefaultPageSizeForMeshModelComponents
		}
	}
	pagestr := r.URL.Query().Get("page")
	page, _ := strconv.Atoi(pagestr)
	if page == 0 {
		page = 1
	}
	offset := (page - 1) * limit

	// TODO: remove this hardcoding. get this from API request params
   res, _ := h.registryManager.GetEntities(&v1alpha1.PolicyFilter{
		Kind: name,
		SubType: r.URL.Query().Get("subtype"),
		Version:  r.URL.Query().Get("version"),
		ModelName: typ,
		APIVersion:  r.URL.Query().Get("apiversion"),
		Limit:    limit,
		Offset:   offset,
		OrderOn:  r.URL.Query().Get("order"),
		Sort:     r.URL.Query().Get("sort"),

	})

	// var policies []v1alpha1.PolicyDefinition
	var policy v1alpha1.PolicyDefinition
	for _, r := range res {
		policy, _ = r.(v1alpha1.PolicyDefinition)
	}

	// evaluate all the rego policies in the policies directory
	networkPolicy, err := policies.RegoPolicyHandler(context.Background(), policy.Expression, "data.network_policy", body)
	if err != nil {
		h.log.Error(ErrResolvingRegoRelationship(err))
		http.Error(rw, ErrResolvingRegoRelationship(err).Error(), http.StatusInternalServerError)
		return
	}

	// write the response
	ec := json.NewEncoder(rw)
	err = ec.Encode(networkPolicy)
	if err != nil {
		h.log.Error(ErrEncoding(err, "networkPolicy response"))
		http.Error(rw, ErrEncoding(err, "networkPolicy response").Error(), http.StatusInternalServerError)
		return
	}
}
