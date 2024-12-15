package k8s

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestUpdateNodeAnnotation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	annotation := map[string]string{
		"scitix.ai/sichek": "",
	}

	client, err := NewClient()
	if err != nil {
		t.Fatalf("failed to create k8s client: %v", err)
	}
	err = client.UpdateNodeAnnotation(ctx, annotation)
	if err != nil {
		t.Errorf("failed to update node annotation to %v: %v", annotation, err)
	}
	node, err := client.GetCurrNode(ctx)
	if err != nil {
		t.Errorf("failed to get node: %v", err)
	}
	fmt.Printf("new annotation: %s", node.Annotations["scitix.ai/sichek"])
}
