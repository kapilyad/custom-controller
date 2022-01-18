package main

import (
	"context"
	"fmt"
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	appsinformers "k8s.io/client-go/informers/apps/v1"
	"k8s.io/client-go/kubernetes"
	appslisters "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type controller struct {
	clientset   kubernetes.Interface
	lister      appslisters.DeploymentLister
	cacheSynced cache.InformerSynced
	queue       workqueue.RateLimitingInterface
}

func newController(clientset kubernetes.Interface, informer appsinformers.DeploymentInformer) *controller {
	c := &controller{
		clientset:   clientset,
		lister:      informer.Lister(),
		cacheSynced: informer.Informer().HasSynced,
		queue:       workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "custom-controller"),
	}

	informer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    c.handleAdd,
			DeleteFunc: c.handleDelete,
		},
	)

	return c
}

func (c *controller) run(ch <-chan struct{}) {
	fmt.Println("starting controller")
	if !cache.WaitForCacheSync(ch, c.cacheSynced) {
		fmt.Print("error occured while syncing cache\n")
	}
	go wait.Until(c.worker, 1*time.Second, ch)
	<-ch // this function will not return until this channel has something
}

func (c *controller) worker() {
	for c.processItem() {

	}

}

func (c *controller) processItem() bool {
	item, shutdown := c.queue.Get()
	if shutdown {
		return false
	}
	key, err := cache.MetaNamespaceKeyFunc(item)
	if err != nil {
		fmt.Printf("getting key from cache %s\n", err.Error())
		return false
	}
	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		fmt.Printf("splitting ns and name from cache %s\n", err.Error())
		return false
	}
	err = c.syncDeployment(ns, name)
	if err != nil {
		fmt.Printf("syncing cache %s\n", err.Error())
	}
	return true
}

func (c *controller) syncDeployment(ns string, name string) error {
	ctx := context.Background()

	//get deployment

	dep, err := c.lister.Deployments(ns).Get(name)
	if err != nil {
		fmt.Printf("getting the deployment from the list %s\n", err.Error())
		return err
	}
	deploymentClient := c.clientset.AppsV1().Deployments(ns)

	annotations := dep.DeepCopy().Annotations

	value, ok := annotations["add-deployment-name-label"]
	if ok && value == "True" {
		if dep.Labels == nil {
			dep.Labels = make(map[string]string)
		}
		dep.Labels["deployment-name"] = dep.Name
		_, err := deploymentClient.Update(ctx, dep, v1.UpdateOptions{})
		if err != nil {
			fmt.Println("update failed", err)
			return err
		}
		fmt.Println("label is added")
	}
	return nil
}

func (c *controller) handleDelete(obj interface{}) {
	fmt.Println("Delete was called")
	c.queue.Add(obj)
}

func (c *controller) handleAdd(obj interface{}) {
	fmt.Println("Add was called")
	c.queue.Add(obj)
}
