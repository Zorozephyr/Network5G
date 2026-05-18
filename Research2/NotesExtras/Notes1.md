DSCP: Differentiation Services Code point
It is a field in IPV4 or IPV6 used for QOS(Quality Of Service)
6 bit value whihc decides priority.For example: High DSCP value for voice packets and low DSCP value for file download. In SMARTNIC Pipeline, it can rewrite this value to a higher priority if required

NAT Translation: Private to Public IP and vice versa

Control Plane Session Setup:
Phone talks to SMF to establish a session, resulting in a TEID being assigned to it. The SMF sends a message to the UPF software running on the servers CPU. This happens over a protocol called PFCP. The UPF software uses an api called P4Runtime to talk to Smart NIC and adds a match + action here

A descriptor rig is a shared To do list that lives in computer's ram. It is how CPU and NIC talk to each other without constantly interupting.Descriptor is a data buffer, which has address, status(Empty-> ready for NIC), Full(ready for CPU). The DMA and descriptor ring only comes into play if its slow path. In fast path, the CPU is just a fallback

DOCA - is Cuda for DPU. Bluefield DPU silicom is the physical chip on a smart nic that contains 8-16 cores running their own full linux OS seperate from the OS.A ConnectX 200/400GBps network engine for wire speed RDMA capable packet I/O, dedicated hardware accelarators for crypto,regex, decompression and flow matching.DPU sits between Netowkr and HostCPU and is fundamentally different from eBPF/XDP which still executes on the host CPU.

A hardware accelerator is a specialized piece of computing hardware designed to perform specific functions more efficiently than a general-purpose CPU.

DOCA Libraries: C/C+= libraries mapping directly to hardware accelarators. 
    DOCA Flow Programs the hardware flow pipeline(the eswitch)....Defines the match and action rules.
    DOCA Crypto offloads IPSec and TLS to dedicated crypto engines
    DOCA DMA -> DPU Memory to Host Memory(Bypass kernal)..Enables DPU to read/write host ram
    DOCA GPUNetIO -> DPU memory to GPU memory..Used for AI Inference
    DOCA Regex
    DOCA Compress
    DOCA Telemetry

Feature,DPDK,SmartNIC,DPU,SuperNIC
Type,Software,Hardware,Hardware,Hardware
Primary Job,Software Kernel Bypass,Hardware Task Offload,Infrastructure Isolation,AI GPU-to-GPU Fabric
Control Plane,Host OS,Host OS,Local DPU OS,Host/Fabric Controller
Key Use Case,"NFV, Fast packet processing","VSwitch offload, telemetry","Cloud multi-tenancy, Zero-trust",Generative AI training clusters

