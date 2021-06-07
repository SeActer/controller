package main

import (
	"flag"
	"github.com/seacter/controller/pkg/client/clientset/versioned"
	"github.com/seacter/controller/pkg/client/informers/externalversions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"k8s.io/klog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

func initClient() (*kubernetes.Clientset, *rest.Config, error) {
	var err error
	var config *rest.Config

	var kubeconfig *string

	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "[可选]kubeconfig的绝对路径")
	} else {
		kubeconfig = flag.String("./config", "", "kubeconfig的绝对路径")
	}
	flag.Parse()
	//初始化 rest.Config对象
	if config, err = rest.InClusterConfig(); err != nil {
		if config, err = clientcmd.BuildConfigFromFlags("", *kubeconfig); err != nil {
			panic(err.Error())
		}
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatal(err)
	}
	return clientset, config, nil
}

func setupSignalHandler() (stopCh <-chan struct{}) {
	stop := make(chan struct{})
	c := make(chan os.Signal, 2)
	signal.Notify(c, []os.Signal{os.Interrupt, syscall.SIGTERM}...)
	go func() {
		<-c
		close(stop)
		<-c
		os.Exit(1)
	}()
	return stop

}

func main() {
	flag.Parse()
	_, config, err := initClient()
	if err != nil {
		klog.Fatal(err)
	}

	//实例化一个crontab的clientset
	crontabClientSet, err := versioned.NewForConfig(config)
	if err != nil {
		klog.Fatal(err)
	}

	stopCh := setupSignalHandler()

	//实例化Crontab 的 informerFactory工厂类
	shardInformerFactory := externalversions.NewSharedInformerFactory(crontabClientSet, time.Second*30)



	//实例化crontab控制器
	controller := NewController(shardInformerFactory.Stable().V1beta1().CronTabs())
	//启动informer，执行listandwatch
	go shardInformerFactory.Start(stopCh)
	//启动控制循环
	if err := controller.Run(1,stopCh); err != nil{
		klog.Fatal(err)
	}


}
