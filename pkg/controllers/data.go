package controllers

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func addData(systemNamespace string, appCtx *appContext) error {
	// TBD: Fill in with resources that need to be added on init, such as the Federation PrometheusRule
	return appCtx.Apply.
		WithSetID("helm-locker-bootstrap-data").
		WithDynamicLookup().
		WithNoDeleteGVK().
		ApplyObjects(&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: systemNamespace,
			},
		})
}
