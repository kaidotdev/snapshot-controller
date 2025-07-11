package routes

import (
	"fmt"
	"log/slog"
	"net/http"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

func Read(dynamicClient *dynamic.DynamicClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		namespace := r.PathValue("namespace")
		group := r.PathValue("group")
		version := r.PathValue("version")
		kind := r.PathValue("kind")
		name := r.PathValue("name")

		gvr := schema.GroupVersionResource{
			Group:    group,
			Version:  version,
			Resource: kind + "s",
		}

		u, err := dynamicClient.Resource(gvr).Namespace(namespace).Get(r.Context(), name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				http.NotFound(w, r)
				return
			}
			slog.Error(fmt.Sprintf("failed to get resource: %s", err))
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		b, err := u.MarshalJSON()
		if err != nil {
			slog.Error(fmt.Sprintf("failed to marshal json: %s", err))
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(b)
	}
}
