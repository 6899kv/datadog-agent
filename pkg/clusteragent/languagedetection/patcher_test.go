// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build kubeapiserver

package languagedetection

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/DataDog/datadog-agent/pkg/languagedetection/util"
	pbgo "github.com/DataDog/datadog-agent/pkg/proto/pbgo/process"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func TestGetContainersLanguagesFromPodDetail(t *testing.T) {

	lp := &LanguagePatcher{
		k8sClient: nil,
	}

	containerDetails := []*pbgo.ContainerLanguageDetails{
		{
			ContainerName: "mono-lang",
			Languages: []*pbgo.Language{
				{Name: "java"},
			},
		},
		{
			ContainerName: "bi-lang",
			Languages: []*pbgo.Language{
				{Name: "java"},
				{Name: "cpp"},
			},
		},
		{
			ContainerName: "tri-lang",
			Languages: []*pbgo.Language{
				{Name: "java"},
				{Name: "go"},
				{Name: "python"},
			},
		},
	}

	podLanguageDetails := &pbgo.PodLanguageDetails{
		Namespace:        "default",
		ContainerDetails: containerDetails,
		Ownerref: &pbgo.KubeOwnerInfo{
			Id:   "dummyId",
			Kind: "replicaset",
			Name: "dummyrs-2342347",
		},
	}

	containerslanguages := lp.getContainersLanguagesFromPodDetail(podLanguageDetails)

	expectedContainersLanguages := util.NewContainersLanguages()

	expectedContainersLanguages.GetOrInitializeLanguageset("mono-lang").Parse("java")
	expectedContainersLanguages.GetOrInitializeLanguageset("bi-lang").Parse("java,cpp")
	expectedContainersLanguages.GetOrInitializeLanguageset("tri-lang").Parse("java,go,python")

	assert.True(t, reflect.DeepEqual(containerslanguages, expectedContainersLanguages))
}

func TestGetOwnersLanguages(t *testing.T) {
	lp := &LanguagePatcher{
		k8sClient: dynamicfake.NewSimpleDynamicClient(runtime.NewScheme()),
	}

	defaultNs := "default"
	customNs := "custom"

	podALanguageDetails := &pbgo.PodLanguageDetails{
		Namespace: defaultNs,
		Name:      "pod-a",
		ContainerDetails: []*pbgo.ContainerLanguageDetails{
			{
				ContainerName: "container-1",
				Languages: []*pbgo.Language{
					{Name: "java"},
					{Name: "cpp"},
					{Name: "go"},
				},
			},
			{
				ContainerName: "container-2",
				Languages: []*pbgo.Language{
					{Name: "java"},
					{Name: "python"},
				},
			},
		},
		InitContainerDetails: []*pbgo.ContainerLanguageDetails{
			{
				ContainerName: "container-3",
				Languages: []*pbgo.Language{
					{Name: "java"},
					{Name: "cpp"},
				},
			},
			{
				ContainerName: "container-4",
				Languages: []*pbgo.Language{
					{Name: "java"},
					{Name: "python"},
				},
			},
		},
		Ownerref: &pbgo.KubeOwnerInfo{
			Id:   "dummyId-1",
			Kind: "replicaset",
			Name: "dummyrs-1-2342347",
		},
	}

	podBLanguageDetails := &pbgo.PodLanguageDetails{
		Namespace: customNs,
		Name:      "pod-b",
		ContainerDetails: []*pbgo.ContainerLanguageDetails{
			{
				ContainerName: "container-5",
				Languages: []*pbgo.Language{
					{Name: "python"},
					{Name: "cpp"},
					{Name: "go"},
				},
			},
			{
				ContainerName: "container-6",
				Languages: []*pbgo.Language{
					{Name: "java"},
					{Name: "ruby"},
				},
			},
		},
		InitContainerDetails: []*pbgo.ContainerLanguageDetails{
			{
				ContainerName: "container-7",
				Languages: []*pbgo.Language{
					{Name: "java"},
					{Name: "cpp"},
				},
			},
			{
				ContainerName: "container-8",
				Languages: []*pbgo.Language{
					{Name: "java"},
					{Name: "python"},
				},
			},
		},
		Ownerref: &pbgo.KubeOwnerInfo{
			Id:   "dummyId-2",
			Kind: "replicaset",
			Name: "dummyrs-2-2342347",
		},
	}

	mockRequestData := &pbgo.ParentLanguageAnnotationRequest{
		PodDetails: []*pbgo.PodLanguageDetails{
			podALanguageDetails,
			podBLanguageDetails,
		},
	}

	expectedContainersLanguagesA := util.NewContainersLanguages()

	expectedContainersLanguagesA.GetOrInitializeLanguageset("container-1").Parse("java,cpp,go")
	expectedContainersLanguagesA.GetOrInitializeLanguageset("container-2").Parse("java,python")
	expectedContainersLanguagesA.GetOrInitializeLanguageset("init.container-3").Parse("java,cpp")
	expectedContainersLanguagesA.GetOrInitializeLanguageset("init.container-4").Parse("java,python")

	expectedContainersLanguagesB := util.NewContainersLanguages()

	expectedContainersLanguagesB.GetOrInitializeLanguageset("container-5").Parse("python,cpp,go")
	expectedContainersLanguagesB.GetOrInitializeLanguageset("container-6").Parse("java,ruby")
	expectedContainersLanguagesB.GetOrInitializeLanguageset("init.container-7").Parse("java,cpp")
	expectedContainersLanguagesB.GetOrInitializeLanguageset("init.container-8").Parse("java,python")

	expectedOwnersLanguages := &OwnersLanguages{
		NewNamespacedOwnerReference("apps/v1", "deployment", "dummyrs-1", "dummyId-1", "default"): expectedContainersLanguagesA,
		NewNamespacedOwnerReference("apps/v1", "deployment", "dummyrs-2", "dummyId-2", "custom"):  expectedContainersLanguagesB,
	}

	actualOwnersLanguages := lp.getOwnersLanguages(mockRequestData)

	assert.True(t, reflect.DeepEqual(expectedOwnersLanguages, actualOwnersLanguages))

}

func TestGetUpdatedOwnerAnnotations(t *testing.T) {
	lp := &LanguagePatcher{
		k8sClient: nil,
	}

	mockContainersLanguages := util.NewContainersLanguages()
	mockContainersLanguages.GetOrInitializeLanguageset("container-1").Parse("cpp,java,python")
	mockContainersLanguages.GetOrInitializeLanguageset("container-2").Parse("python,ruby")
	mockContainersLanguages.GetOrInitializeLanguageset("container-3").Parse("cpp")
	mockContainersLanguages.GetOrInitializeLanguageset("container-4").Parse("")

	// Case of existing annotations
	mockCurrentAnnotations := map[string]string{
		"annotationkey1": "annotationvalue1",
		"annotationkey2": "annotationvalue2",
		"apm.datadoghq.com/container-1.languages": "java,python",
		"apm.datadoghq.com/container-2.languages": "cpp",
	}

	expectedUpdatedAnnotations := map[string]string{
		"annotationkey1": "annotationvalue1",
		"annotationkey2": "annotationvalue2",
		"apm.datadoghq.com/container-1.languages": "cpp,java,python",
		"apm.datadoghq.com/container-2.languages": "cpp,python,ruby",
		"apm.datadoghq.com/container-3.languages": "cpp",
	}

	expectedAddedLanguages := 4

	actualUpdatedAnnotations, actualAddedLanguages := lp.getUpdatedOwnerAnnotations(mockCurrentAnnotations, mockContainersLanguages)

	assert.Equal(t, expectedAddedLanguages, actualAddedLanguages)
	assert.Equal(t, expectedUpdatedAnnotations, actualUpdatedAnnotations)

	// Case of non-existing annotations
	mockCurrentAnnotations = nil

	expectedUpdatedAnnotations = map[string]string{
		"apm.datadoghq.com/container-1.languages": "cpp,java,python",
		"apm.datadoghq.com/container-2.languages": "python,ruby",
		"apm.datadoghq.com/container-3.languages": "cpp",
	}

	expectedAddedLanguages = 6

	actualUpdatedAnnotations, actualAddedLanguages = lp.getUpdatedOwnerAnnotations(mockCurrentAnnotations, mockContainersLanguages)

	assert.Equal(t, expectedAddedLanguages, actualAddedLanguages)
	assert.Equal(t, expectedUpdatedAnnotations, actualUpdatedAnnotations)

}

func TestPatchOwner(t *testing.T) {

	mockK8sClient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())

	lp := &LanguagePatcher{
		k8sClient: mockK8sClient,
	}

	deploymentName := "test-deployment"
	ns := "test-namespace"
	gvr := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}

	namespacedOwnerReference := NewNamespacedOwnerReference("apps/v1", "deployment", deploymentName, "uid-dummy", ns)

	mockContainersLanguages := util.NewContainersLanguages()
	mockContainersLanguages.GetOrInitializeLanguageset("container-1").Parse("cpp,java,python")
	mockContainersLanguages.GetOrInitializeLanguageset("container-2").Parse("python,ruby")
	mockContainersLanguages.GetOrInitializeLanguageset("container-3").Parse("cpp")
	mockContainersLanguages.GetOrInitializeLanguageset("container-4").Parse("")

	// Create target deployment
	deploymentObject := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":      deploymentName,
				"namespace": ns,
				"annotations": map[string]interface{}{
					"annotationkey1": "annotationvalue1",
					"annotationkey2": "annotationvalue2",
					"apm.datadoghq.com/container-1.languages": "java,python",
					"apm.datadoghq.com/container-2.languages": "cpp",
				},
			},
			"spec": map[string]interface{}{},
		},
	}
	_, err := mockK8sClient.Resource(gvr).Namespace(ns).Create(context.TODO(), deploymentObject, metav1.CreateOptions{})
	assert.NoError(t, err)

	// Apply the patch
	assert.NoError(t, lp.patchOwner(&namespacedOwnerReference, mockContainersLanguages))

	// Check the patch
	got, err := lp.k8sClient.Resource(gvr).Namespace(ns).Get(context.TODO(), deploymentName, metav1.GetOptions{})
	assert.NoError(t, err)

	annotations, found, err := unstructured.NestedStringMap(got.Object, "metadata", "annotations")
	assert.NoError(t, err)
	assert.True(t, found)

	expectedAnnotations := map[string]string{
		"apm.datadoghq.com/container-1.languages": "cpp,java,python",
		"apm.datadoghq.com/container-2.languages": "cpp,python,ruby",
		"apm.datadoghq.com/container-3.languages": "cpp",
		"annotationkey1": "annotationvalue1",
		"annotationkey2": "annotationvalue2",
	}

	assert.True(t, reflect.DeepEqual(expectedAnnotations, annotations))
}

func TestPatchAllOwners(t *testing.T) {
	mockK8sClient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())

	lp := &LanguagePatcher{
		k8sClient: mockK8sClient,
	}

	gvr := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}

	// Mock definition for deployment A
	deploymentAName := "test-deployment-A"
	nsA := "test-namespace-A"

	podALanguageDetails := &pbgo.PodLanguageDetails{
		Namespace: nsA,
		Name:      "pod-a",
		ContainerDetails: []*pbgo.ContainerLanguageDetails{
			{
				ContainerName: "container-1",
				Languages: []*pbgo.Language{
					{Name: "java"},
					{Name: "cpp"},
					{Name: "python"},
				},
			},
			{
				ContainerName: "container-2",
				Languages: []*pbgo.Language{
					{Name: "ruby"},
					{Name: "python"},
				},
			},
		},
		InitContainerDetails: []*pbgo.ContainerLanguageDetails{
			{
				ContainerName: "container-3",
				Languages: []*pbgo.Language{
					{Name: "cpp"},
				},
			},
			{
				ContainerName: "container-4",
				Languages:     []*pbgo.Language{},
			},
		},
		Ownerref: &pbgo.KubeOwnerInfo{
			Id:   "dummyId-1",
			Kind: "replicaset",
			Name: "test-deployment-A-2342347",
		},
	}

	// Mock definition for deployment B
	deploymentBName := "test-deployment-B"
	nsB := "test-namespace-B"

	podBLanguageDetails := &pbgo.PodLanguageDetails{
		Namespace: nsB,
		Name:      "pod-b",
		ContainerDetails: []*pbgo.ContainerLanguageDetails{
			{
				ContainerName: "container-1",
				Languages: []*pbgo.Language{
					{Name: "python"},
				},
			},
			{
				ContainerName: "container-2",
				Languages: []*pbgo.Language{
					{Name: "golang"},
				},
			},
		},
		InitContainerDetails: []*pbgo.ContainerLanguageDetails{
			{
				ContainerName: "container-3",
				Languages: []*pbgo.Language{
					{Name: "cpp"},
					{Name: "java"},
				},
			},
			{
				ContainerName: "container-4",
				Languages:     []*pbgo.Language{},
			},
		},
		Ownerref: &pbgo.KubeOwnerInfo{
			Id:   "dummyId-2",
			Kind: "replicaset",
			Name: "test-deployment-B-2342347",
		},
	}

	// Create target deployment A
	deploymentAObject := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":      deploymentAName,
				"namespace": nsA,
				"annotations": map[string]interface{}{
					"annotationkey1": "annotationvalue1",
					"annotationkey2": "annotationvalue2",
					"apm.datadoghq.com/container-1.languages": "java,python",
					"apm.datadoghq.com/container-2.languages": "cpp",
				},
			},
			"spec": map[string]interface{}{},
		},
	}
	_, err := mockK8sClient.Resource(gvr).Namespace(nsA).Create(context.TODO(), deploymentAObject, metav1.CreateOptions{})
	assert.NoError(t, err)

	// Create target deployment B
	deploymentBObject := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":      deploymentBName,
				"namespace": nsB,
			},
			"spec": map[string]interface{}{},
		},
	}
	_, err = mockK8sClient.Resource(gvr).Namespace(nsB).Create(context.TODO(), deploymentBObject, metav1.CreateOptions{})
	assert.NoError(t, err)

	mockRequestData := &pbgo.ParentLanguageAnnotationRequest{
		PodDetails: []*pbgo.PodLanguageDetails{
			podALanguageDetails,
			podBLanguageDetails,
		},
	}

	// Apply the patches to all owners
	lp.PatchAllOwners(mockRequestData)

	// Check the patch of owner A
	got, err := lp.k8sClient.Resource(gvr).Namespace(nsA).Get(context.TODO(), deploymentAName, metav1.GetOptions{})
	assert.NoError(t, err)

	annotations, found, err := unstructured.NestedStringMap(got.Object, "metadata", "annotations")
	assert.NoError(t, err)
	assert.True(t, found)

	expectedAnnotationsA := map[string]string{
		"apm.datadoghq.com/container-1.languages":      "cpp,java,python",
		"apm.datadoghq.com/container-2.languages":      "cpp,python,ruby",
		"apm.datadoghq.com/init.container-3.languages": "cpp",
		"annotationkey1": "annotationvalue1",
		"annotationkey2": "annotationvalue2",
	}

	fmt.Println(expectedAnnotationsA)
	fmt.Println(annotations)
	assert.True(t, reflect.DeepEqual(expectedAnnotationsA, annotations))

	// Check the patch of owner B
	got, err = lp.k8sClient.Resource(gvr).Namespace(nsB).Get(context.TODO(), deploymentBName, metav1.GetOptions{})
	assert.NoError(t, err)

	annotations, found, err = unstructured.NestedStringMap(got.Object, "metadata", "annotations")
	assert.NoError(t, err)
	assert.True(t, found)

	expectedAnnotationsB := map[string]string{
		"apm.datadoghq.com/container-1.languages":      "python",
		"apm.datadoghq.com/container-2.languages":      "golang",
		"apm.datadoghq.com/init.container-3.languages": "cpp,java",
	}

	assert.True(t, reflect.DeepEqual(expectedAnnotationsB, annotations))

}
