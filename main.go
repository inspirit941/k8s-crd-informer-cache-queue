package main

import (
	"flag"
	"fmt"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"log"
	"os"
	"path/filepath"
	"time"
)

func main() {
	var kubeconfig *string
	if home, err := os.UserHomeDir(); err != nil {
		// containerize할 경우 이 코드는 동작하지 않음. 따라서 kubeconfig가 생성되지 않음.
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		log.Printf("building config from flags, trying to build inclusterconfig. %s", err.Error())
		// kubeconfig 파일이 없는 경우
		config, err = rest.InClusterConfig() // kubernetes pod에 mount된 secret Account를 사용
		if err != nil {
			log.Printf("error %s building inclusterconfig: %s", err.Error())
		}
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Printf("error %s, creating clientset\n", err.Error())
	}

	// factory 생성.
	// 2번째 arg인 30 sec의 의미? -> cluster와 inmemory store 정보 resync 수행하는 주기.
	// - 최초에 list로 inmemory store에 값을 채워넣으면, 그 다음부터는 watch로 상태변화를 체크한다.
	//   이 때, 모종의 이유로 watch 요청이 실패할 수 있다. 그러면 보통은 informer가 new watch request를 생성함.
	//   그럼에도 클러스터에서 해당 리소스를 더 이상 조회할 수 없는 경우.. inmemory store 정보를 cluster 정보와 resync한다.
	//   resync 수행하는 시간이 두 번째 arg. 보통 10~20 min으로 설정한다.
	informerFactory := informers.NewSharedInformerFactory(clientset, 30*time.Second)

	// 특정 리소스에 해당하는 informer를 생성할 수 있다.
	podInformer := informerFactory.Core().V1().Pods()

	// register some functions to listen certain kind of events.
	// cluster에서 정보 조회 -> inmemory store에 값을 업데이트할 때 실행됨.
	//  -> 특정 이벤트가 발생할 때, 또는 resync가 수행될 때 invoke된다.
	//     따라서, 상태변화가 없었는데 resync 때문에 로직이 수행되는 걸 막기 위해서는 ResourceVersion을 비교하면 된다.
	podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(new interface{}) {
			fmt.Println("add was called")
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			// oldObj와 newObj의 resourceVersion을 비교하면, 상태변화 없었는데 resync로 호출된 게 아닌지 체크 가능.
			fmt.Println("update was called")
		},
		DeleteFunc: func(obj interface{}) {
			fmt.Println("delete was called")
		},
	})

	// informer의 inmemory store를 initialize하는 명령어. apiserver에 list 요청 보내서 필요한 정보를 가져온다.
	informerFactory.Start(wait.NeverStop)

	// 한 번 list로 inmemory store 정보에 값을 채웠으면, 그 다음부터는 watch 요청을 보냄.
	informerFactory.WaitForCacheSync(wait.NeverStop)

	// informer에서 list해오는 값은 api server에 조회하는 것이 아님. inmemory에서 조회해온다
	// 따라서, inmemory store에서 조회해온 resource를 수정하는 건 바람직하지 않다.
	//  변경해야 한다면 deepCopy한 object를 사용해야 함.
	pod, err := podInformer.Lister().Pods("default").Get("default")
	fmt.Println(pod)

	// NewSharedInformerFactory 함수는 모든 ns의 모든 resource 정보를 저장함.
	// 특정 ns나 resource만 관리하게 하고 싶으면 NewSharedInformerFactoryWithOptions 함수를 쓰면 된다.
	informers.NewSharedInformerFactoryWithOptions(clientset, 30*time.Second,
		informers.WithNamespace("namespace"),
		informers.WithTweakListOptions(func(options *v1.ListOptions) {
			options.LabelSelector = "test"
		}),
	)

	// 만약 event handler 로직이 내부적으로 실패할 경우?
	// AddFunc, UpdateFunc 등에서 실제로 비즈니스 로직을 전부 수행하는 대신 queue (workqueue)에 집어넣는다.
	// 비즈니스 로직을 수행하는 goroutine을 따로 만들고, goroutine에서 queue를 consume하는 방식을 권장함.
	//   이렇게 되면, 비즈니스 로직이 실패할 경우 queue에 해당 리소스를 다시 집어넣으면 된다.
	//             로직이 성공하면 queue의 리소스를 Done 처리.
}
