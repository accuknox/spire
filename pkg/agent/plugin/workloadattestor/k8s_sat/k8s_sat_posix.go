package k8ssat

import (
	"context"
	"fmt"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/hashicorp/go-hclog"
	review "k8s.io/api/authentication/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	SourceTypeServiceAccountName = "serviceAccountName"
	SourceTypePodName            = "podName"
)

func (p *Plugin) validateServiceAccountToken(token string, log hclog.Logger) ([]string, error) {
	var (
		podName, podNs string
		selectors      []string
		err            error
	)

	if token == "" {
		log.Error("empty token")
		return selectors, fmt.Errorf("empty token received ")
	}

	result, err := p.client.AuthenticationV1().TokenReviews().Create(context.Background(), &review.TokenReview{
		Spec: review.TokenReviewSpec{
			Token: token,
		},
	}, metav1.CreateOptions{})

	if err != nil || !result.Status.Authenticated {
		log.Error("tokenReview failed: ", err)
		return selectors, fmt.Errorf("tokenReview failed")
	}

	for key, value := range result.Status.User.Extra {
		if strings.Contains(key, "pod-name") {
			podName = value[0]
			break
		}
	}

	jwtToken, _, err := jwt.NewParser().ParseUnverified(token, jwt.MapClaims{})
	if err != nil {
		log.Error("token decoding failed ", err)
		return selectors, err
	}

	mapClaims := jwtToken.Claims.(jwt.MapClaims)
	k8sIOMap, ok := mapClaims["kubernetes.io"].(map[string]any)
	if !ok {
		log.Warn("kubernetes.io not found in decoded token")
		for key, value := range mapClaims {
			if strings.Contains(key, "namespace") {
				podNs = value.(string)
				break
			}
		}
	} else {
		podNs = k8sIOMap["namespace"].(string)
	}

	if podNs == "" && podName == "" {
		log.Error("pod name and namespace not found in token")
		return selectors, fmt.Errorf("pod name and namespace not found in token")
	}

	if podName == "" {
		subSplit := strings.Split(mapClaims["sub"].(string), ":")
		saName := subSplit[len(subSplit)-1]
		selectors, err = p.getSelectorValues(SourceTypeServiceAccountName, saName, podNs)
	} else {
		selectors, err = p.getSelectorValues(SourceTypePodName, podName, podNs)
	}

	return selectors, err
}

func (p *Plugin) getSelectorValues(
	sourceType, sourceName, namespace string,
) ([]string, error) {

	podList, err := p.getPodDetails(sourceName, namespace)
	if err != nil {
		return nil, err
	}

	var selectorValues []string
	switch sourceType {
	case SourceTypePodName:
		for _, pod := range podList.Items {
			if pod.Name == sourceName {
				selectorValues = getSelectorValues(&pod)
			}
		}

	case SourceTypeServiceAccountName:
		for _, pod := range podList.Items {
			if pod.Spec.ServiceAccountName == sourceName {
				selectorValues = getSelectorValues(&pod)
			}
		}
	default:
		return nil, fmt.Errorf("unknown source type: %s", sourceType)
	}

	return selectorValues, nil

}

func (p *Plugin) getPodDetails(podName, namespace string) (*v1.PodList, error) {

	if podName == "" && namespace != "" {
		return p.client.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{})
	}
	pod, err := p.client.CoreV1().Pods(namespace).Get(context.Background(), podName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return &v1.PodList{Items: []v1.Pod{*pod}}, nil
}

func getSelectorValues(pod *v1.Pod) []string {

	selectorValues := []string{
		fmt.Sprintf("sa:%s", pod.Spec.ServiceAccountName),
		fmt.Sprintf("ns:%s", pod.Namespace),
		fmt.Sprintf("pod-uid:%s", pod.UID),
		fmt.Sprintf("pod-name:%s", pod.Name),
	}
	for k, v := range pod.Labels {
		selectorValues = append(selectorValues, fmt.Sprintf("pod-label:%s:%s", k, v))
	}
	return selectorValues
}
