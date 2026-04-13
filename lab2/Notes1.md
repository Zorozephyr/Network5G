sk_buff is the main struct used for network packet processing
Has 4 sections/4pointers:
    1.head 
    2.data
    3.tail
    4.end

                                ---------------
                               | sk_buff       |
                                ---------------
   ,---------------------------  + head
  /          ,-----------------  + data
 /          /      ,-----------  + tail
|          |      |            , + end
|          |      |           |
v          v      v           v
 -----------------------------------------------
| headroom | data |  tailroom | skb_shared_info |
 -----------------------------------------------
                               + [page frag]
                               + [page frag]
                               + [page frag]
                               + [page frag]       ---------
                               + frag_list    --> | sk_buff |
                                                   ---------


headRoom gets expanded as it passes through each layer like MAC header by ethernet, IP header, TCP header etc

Data contains actual payload. Sometimes packet might be extremely large, therefore it wont be viable to store data sequentially in memory, hence page_frag points to each of those memory points with the extra data.

TailRoom is extra area for appending footers like Ethernet Framer Check Sequence

sk_buff_head points to head of doubly linked queue where each network packet is added. head->next point to first packet of the queue
head->prev points to last packet of the queue

qlen -> Tracks length of queue
It also has a lock(spinlock), a mutex is not used because a hardware interuppt is done by NIC to quickly grab the packet and put in queue. If mutex is used the interupt handler will go to sleep freezing the system, hence a spinlock which continously runs i used.

,-----------------------------------------------------.
       |                                                     |
 +------------+        +-----------+        +-----------+    |
 |sk_buff_head|  next  | sk_buff 1 |  next  | sk_buff 2 |    |
 | qlen: 2    | -----> |           | -----> |           | ---'
 | lock: 0    |        |           |        |           |
 |            | <----- |           | <----- |           | <---.
 +------------+  prev  +-----------+  prev  +-----------+     |
       |                                                      |
       `------------------------------------------------------'

sk_buff struct u can check in linux

But a few important parts of it are the following:
1. net_device:
    dev pointer answers the question which hardware currenlty own the packet. The packet arrives at actual hardware chip. Dev says eth0. The kernal moves packet to virtual bridge that connects all the containers and dev point is updated to docker0.The bridge hands it to specific containers virtual netwokr interface and it changes to veth1234
2. control_block - char                    cb[48] __aligned(8); 
    Is 48byte memory array
    Eg flow: Ip dept first checks it, then tcp layer. Each of these dept jot downs it temporary data and flags. They use this area as a white board which each layer rewrites
3. skb_clone and  cloned flag
    Imagine you have a massive 1500-byte payload (the package inside the shipping container). Now, imagine a scenario where two different applications need to read that exact same packet at the same time (like when you are running a network sniffer tool while browsing the web).
    Don't copy the package. Instead, just build a second empty sk_buff (a new shipping manifest) and point it to the exact same physical package sitting in memory.
    To prevent chaos, the kernel flips a switch called cloned = 1. This tells everyone: "Hey, multiple people are looking at this data. It is currently READ-ONLY."

sysctl -> Used to modify kernal parameters at runtime. The parameters are in /proc/sys
ioctl -> mechanism for user space programs to interact with device driver and control hardware devices. App can ask Kernal(Only one way communication)
Netlink -> Established 2 way API based communication between user space and kernel space.
           fd = socket(AF_NETLINK//Address intercepted by kernal, SOCK_RAW, NETLINK_GENERIC)
           send()
           recv()
           If the protocol is generic, then there will be another call to query the exact kernal application id
           It uses socket, therefore the kernal can easily broadcast multiple messages at once
           NetlinkMessage ->(MessageHeader(Pid,len,type,flag,seq),payload,Padding)
           Seq can be used to send and recieve multiple messages by matching replies to question sequence number
           Pid is process id of asking program

Flow:
1. NIC reviece packet
2. Device driver generates interupt
3. CPU halts current taks and identifies interupt source using Interupt Vector Table
4. CPU run Interrupt Service Routinue from device layer and passes packet information to driver which process it
5. Driver passes packet to kernal for further processing like routing and TCP IP handling
6. CPU continues processing

The interuppt creates sk_buff structure

Softirq(Soft Interrupts):
    Defers processing of certain tasks from the context of a hardware interupt handler to a later time the CPU is less loaded. Allows the hardwaare interupt handler to return quickly. Softirqs has a kernal thread ksoftirqd that operates on CPU basis.
As discussed in Notes1.md, too many interupts on the queue can paralyze the system..Hence there is a new mechanism called NAPI(New API)....New interupts are not generated until queue finish processing other packets in the queue. Combining interuppt and polling mechanisms.

netif_receive_skb: Once the packet is in memory (inside the sk_buff structure), this function acts as the unboxer. It rips off the outer Ethernet envelope (Layer 2), looks at the protocol flag, and says: "This is an IPv4 packet, send it up to the IPv4 department."

Layer3:
    The layer 3 first does Sanity Checks(Checksum). If it is broken, it drops the package.
    Netfilter(NF_INET_PRE_ROUTING): Checks firewall(iptables or ufw) and confirms if it is blocked or not
    Once the packet survices the firewall, it verifies if the packet is for the same machine or is it meant to be forwarded(ip_route_input)
    If its forwarded(then it enter ip_forward function)...Every packet has TTL and kernal subtracts 1 from it, each time its get forwarded
    If the packet is much larger than MTU..then it gets fragmented, which takes a lot of processing power
