SERVER

cd test/networkdb
env GOOS=linux go build -v testMain.go && docker build -t dockereng/e2e-networkdb .
(only for testkit case) docker push dockereng/e2e-networkdb

Run server: docker service create --name testdb --network net1 --replicas 3 --env TASK_ID="{{.Task.ID}}" -p mode=host,target=8000 dockereng/e2e-networkdb server 8000

CLIENT

cd test/networkdb
Join cluster: docker run -it --network net1 dockereng/e2e-networkdb client join testdb 8000
Join network: docker run -it --network net1 dockereng/e2e-networkdb client join-network testdb 8000 test
Run test: docker run -it --network net1 dockereng/e2e-networkdb client write-delete-unique-keys testdb 8000 test tableBla 3 10
check table: curl "localhost:32768/gettable?nid=test&tname=table_name"
