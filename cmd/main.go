package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"spdytests/bindata"

	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
)

var namespace = "spdytests"

func main() {
	config, err := clientcmd.BuildConfigFromFlags("", os.Getenv("KUBECONFIG"))
	if err != nil {
		log.Fatalf("BuildConfigFromFlags: %v", err)
	}
	config.APIPath = "/api"
	config.NegotiatedSerializer = scheme.Codecs
	config.GroupVersion = &schema.GroupVersion{
		Version: "v1",
	}

	cli, err := rest.RESTClientFor(config)
	if err != nil {
		log.Fatalf("rest.RESTClientFor: %v", err)
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		name, err := createBuilderPod(config)
		if err != nil {
			out := fmt.Sprintf("createBuilderPod: %s\n", err.Error())
			w.Write([]byte(out))
			return
		}

		req := cli.Post().
			Namespace(namespace).
			Resource("pods").
			Name(name).
			SubResource("attach")

		opts := &corev1.PodAttachOptions{
			Stdin: true,
		}
		req.VersionedParams(opts, scheme.ParameterCodec)

		exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
		if err != nil {
			out := fmt.Sprintf("NewSPDYExecutor: %s\n", err.Error())
			w.Write([]byte(out))
			return
		}

		if err := exec.Stream(remotecommand.StreamOptions{
			Stdin: r.Body,
		}); err != nil {
			out := fmt.Sprintf("Stream: %s\n", err.Error())
			w.Write([]byte(out))
			return
		}

		w.Write([]byte("done."))
	})

	log.Print("listening on :8181")
	if err := http.ListenAndServe(":8181", nil); err != nil {
		log.Fatalf("ListenAndServe: %v", err)
	}
}

func createBuilderPod(config *rest.Config) (string, error) {
	content, err := bindata.Asset("assets/bsdtar.json")
	if err != nil {
		return "", err
	}

	pod := &corev1.Pod{}
	if err := json.Unmarshal(content, pod); err != nil {
		return "", err
	}
	podName := fmt.Sprintf("bsdtar-%s", uuid.New().String())

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return "", err
	}

	pod.ObjectMeta.Name = podName
	pod, err = clientset.CoreV1().Pods(namespace).Create(pod)
	if err != nil {
		return "", err
	}

	if err := wait.Poll(time.Second, time.Minute, func() (bool, error) {
		if pod, err = clientset.CoreV1().Pods(namespace).Get(
			podName, metav1.GetOptions{},
		); err != nil {
			return false, nil
		}
		return pod.Status.Phase == corev1.PodRunning, nil
	}); err != nil {
		return "", err
	}

	return podName, nil
}
