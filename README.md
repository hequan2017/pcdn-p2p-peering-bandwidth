# pcdn-p2p-peering-bandwidth
点对点 互拉带宽


## 打包
> gox -osarch="linux/amd64"   -ldflags "-s -w"   -output="server"

> cd peer  && gox -osarch="linux/amd64"   -ldflags "-s -w"   -output="peer"


## server
>  ./server

### 第一台
>  ./peer  --hequanid="321" --serverip="" --serverport=10000 --clientport=9999 --network="eth0" --tohequanid="123"  --time=10     --uploadrate=1 --downloadrate=1

### 第二台
>  ./peer  --hequanid="123" --serverip="" --serverport=10000 --clientport=9999 --network="eth0" --tohequanid="321"  --time=10     --uploadrate=1 --downloadrate=1
