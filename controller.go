package main

import (
	"fmt"
	crdv1beta1 "github.com/seacter/controller/pkg/apis/stable/v1beta1"
	informer "github.com/seacter/controller/pkg/client/informers/externalversions/stable/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"
	"time"
)

type Controller struct {
	informer informer.CronTabInformer
	queue    workqueue.RateLimitingInterface
}

func  NewController(informer informer.CronTabInformer) *Controller {
	controller := &Controller{
		informer: informer,
		queue:    workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "crontab"),
	}
	klog.Info("setting up crontab controller")

	//注册事件监听函数
	informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.onAdd,
		UpdateFunc: controller.onUpdate,
		DeleteFunc: controller.onDelete,
	})
	return controller
}

func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()

	//停止控制器后关掉队列
	defer c.queue.ShutDown()
	klog.Info("start crontab controller")

	if !cache.WaitForCacheSync(stopCh, c.informer.Informer().HasSynced) {
		return fmt.Errorf("timeout waiting for caches to sync")
	}

	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}
	<-stopCh
	klog.Info("Stop crontab controller")
	return nil
}

//处理元素
func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

//实现业务逻辑
func (c *Controller) processNextWorkItem() bool {
	//从workqueue取出一个元素
	obj, shutdown := c.queue.Get()
	if shutdown {
		return false
	}

	//根据key处理逻辑
	err := func(obj interface{}) error {

		//告诉队列 我们已经处理了key
		defer c.queue.Done(obj)
		var ok bool
		var key string
		if key, ok = obj.(string); !ok {
			c.queue.Forget(obj)
			return fmt.Errorf("expected string in workqueue")
		}
		//业务逻辑
		if err := c.syncHandler(key); err != nil {
			return fmt.Errorf("sync error: %v", err)
		}
		c.queue.Forget(key)
		klog.Info("Succeddfully synced %s", key)
		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
	}
	return true
}

//拿到key，然后获取crontab，需要从index获取
func (c *Controller) syncHandler(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	//获取crontab,实际上是从index中获取
	crontab, err := c.informer.Lister().CronTabs(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			//对应的crontab对象已经被删除了
			klog.Warningf("crobtab deleteing: %s/%s", namespace, name)
			return nil
		}
		return err
	}

	klog.Info("crontab try to process: %#v", crontab)

	return nil

}

func (c *Controller) onAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
	}
	c.queue.AddRateLimited(key)

}

func (c *Controller) onUpdate(old, new interface{}) {
	oldObj := old.(*crdv1beta1.CronTab)
	newObj := old.(*crdv1beta1.CronTab)
	//比较资源对象的版本
	if oldObj.ResourceVersion == newObj.ResourceVersion {
		return
	}
	c.onAdd(newObj)
}

func (c *Controller) onDelete(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}
	c.queue.AddRateLimited(key)
}
