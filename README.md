## Informer, Cache and Queue | Kubernetes Informers vs Watch | Basics of client-go Kubernetes Part - 4

- https://youtu.be/soyOjOH-Vjc?si=394vjrwBVjxwyAPM
- 참고 repo: https://github.com/viveksinghggits/lister

Controller는 k8s의 특정 리소스를 listen on 하는 역할. 
- 예컨대 pod라는 리소스를 monitoring해서, pod가 생성되면 자동으로 service를 만들어주는 custom controller를 구상한다고 하자.
- pod라는 리소스가 생성 / 수정 / 삭제된다는 변경 이력을 controller가 어떻게 알 수 있나?


pod resource를 위한 PodInterface에 Watch() 메소드가 있다.
- returns channel. channel에서 모든 종류의 pod event를 제공함. 
- 따라서 pod resource의 상태 변화를 받을 수 있다.

그러나 보통 controller에서는 watch 메소드를 직접 쓰지 않는다. 최소한 우리가 방금 예시로 든 상황에서만큼은 안 쓸 예정.
- **watch 함수는 k8s apiserver에 계속 query를 보내면서 리소스의 상태변화를 체크하는 방식**. 따라서 apiserver에 부하가 크다.

---

informer를 사용한다. informer도 내부적으로는 watch를 사용하지만, efficiently leverages in memory store.
- 최초에 apiserver에 list 요청을 보내서 응답이 오면, store라는 메모리 내 공간에 값을 저장해둔다.
- watch로 상태 변화를 확인하면, store에 값을 업데이트한다.
- 따라서 apiserver에 직접 요청을 보내는 작업은 informer에 일임하고, informer가 관리하는 store에 get / list로 리소스를 조회하는 방식.
  - watch의 경우, api server와 통신이 안 될 경우의 handle 로직을 전부 직접 짜야 함.
  - informer의 경우 기본적인 에러 핸들링이 내장되어 있음

따라서 controller를 생성하고 관리할 거라면 informer는 매우 중요한 컴포넌트.
<br>

can create informer for every group / version / resource.
- 만약 세 개의 group/version/resource 를 관리하려고 informer를 세 개 만들면, api server에 불필요한 부하가 가해지는 건 watch와 똑같아진다.
- 따라서 **sharedInformerFactory** 를 사용한 informer initialize를 보통 사용함.
  - 

