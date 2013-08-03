#!/usr/bin/env python

import os
import sys
import time

import mesos
import mesos_pb2
import redis
import threading
import json
import socket
import logging as logger

#2.6 python logging broken as fuck
level=logger.DEBUG
logger.basicConfig(level=level, format='%(asctime)s %(levelname)s:%(message)s')

def init_data(redis):
    job={'name':'testapp',
         'req_instances': 0,
         'running_instances': 0,
         'launched_instances': 0,
         'cpu': 1,
         'mem': 32,
         'command': 'url-runner.sh',
         'frontend': 'ec2-54-225-121-86.compute-1.amazonaws.com',  #ideally we use wildcard DNS, someapp.docker_cluster.company.com
         'args': 'https://test_prettyprints.s3.amazonaws.com/webapp-AIOv0.1',
         'base_port':10000}
    pipe = redis.pipeline()
    pipe.hmset('docker_apps_%s' % job['name'], job)
    pipe.hmset('docker_apps_testapp_env', { 'test_key':'test_value'})
    pipe.sadd('docker_apps', job['name'])
    pipe.delete(job['frontend'])
    pipe.delete('docker_apps_%s_tasks' % job['name'])
    pipe.rpush('frontend:%s' % job['frontend'], 'awesomelb')
    pipe.execute()

class DockerScheduler(mesos.Scheduler):
    def __init__(self, executor, r_pool):
        self.executor = executor
        self.tasksLaunched = 0
        self.tasksRunning = {}
        self.tasksFinished = 0
        self.redis = redis.Redis(connection_pool=r_pool)
        init_data(self.redis)

    def registered(self, driver, fid, masterInfo):
        logger.info("Registered with framework ID %s" % fid.value)
        self.driver = driver

    def resourceOffers(self, driver, offers):
        logger.info("Got %d resource offers" % len(offers))
        apps = self.redis.smembers('docker_apps')
        for offer in offers:
            logger.info("Got resource offer %s from %s" % (offer.id.value,offer.hostname))
            offer_cpu=offer.resources[0].scalar.value
            offer_mem=offer.resources[1].scalar.value
            logger.info("Offer is for " +str(offer_cpu) + " CPUS and " + str(offer_mem) + " MB")
            tasks = []
            #In order to fan out the app instasnces we just launch one instance per app per round
            pipe = self.redis.pipeline()
            for app in apps:
                app = self.redis.hgetall('docker_apps_%s' % app )
                #redis converts to floats and protobuf wants ints
                app['cpu'] = int(app['cpu'])
                app['mem'] = int(app['mem'])
                logger.info("Finding resources for %s, needs itself some %s cpu and %s mem" % (app['name'], app['cpu'], app['mem']))
                if (app['running_instances'] < app['req_instances']) and offer_cpu>=app['cpu'] and offer_mem>= app['mem']:
                    logger.info("Attempting to launch %s, current instances: %s, requested instances %s" % (app['name'],app['running_instances'], app['req_instances']))
                    tid = self.tasksLaunched
                    logger.info("Accepting offer on %s to start an instance of %s, task_id %d" % (offer.hostname, app['name'], tid))

                    task = mesos_pb2.TaskInfo()
                    task.task_id.value = str("%s_%s_%s" % (app['name'], offer.hostname, tid))
                    task.slave_id.value = offer.slave_id.value
                    task.name = "task %d" % tid
                    task.executor.MergeFrom(self.executor)
                    app['port'] = int(app['base_port']) + tid
                    app['env'] = self.redis.hgetall("docker_apps_%s_env" % app['name'])
                    task.data = json.dumps(app)

                    cpus = task.resources.add()
                    cpus.name = "cpus"
                    cpus.type = mesos_pb2.Value.SCALAR
                    cpus.scalar.value = app['cpu']

                    mem = task.resources.add()
                    mem.name = "mem"
                    mem.type = mesos_pb2.Value.SCALAR
                    mem.scalar.value = app['mem']

                    self.tasksLaunched += 1
                    tasks.append(task)
                    offer_cpu -= app['cpu']
                    offer_mem -= app['mem']
                    pipe.hincrby('docker_apps_%s' % app['name'], 'running_instances', 1)
                    pipe.hincrby('docker_apps_%s' % app['name'], 'launched_instances', 1)
                    pipe.rpush('docker_apps_%s_tasks' % app['name'], task.task_id.value)
                    pipe.rpush('frontend:%s' % app['frontend'], 'http://%s:%s' % (socket.gethostbyname(offer.hostname), app['port']))
            logger.info("Launching tasks: %s" % tasks)
            driver.launchTasks(offer.id, tasks)
            pipe.execute()

    def statusUpdate(self, driver, update):
        logger.info("Task %s is in state %d" % (update.task_id.value, update.state))
        if update.state == mesos_pb2.TASK_FINISHED:
            self.tasksFinished += 1
            # driver.stop(False)

def monitor(sched):
    logger.info("in MONITOR()")
    while True:
        time.sleep(1)
        #logger.info("Monitor looks around suspicously.."
        apps = sched.redis.smembers('docker_apps')
        for app in apps:
            app = sched.redis.hgetall('docker_apps_%s' % app )
            if int(app['running_instances']) > int(app['req_instances']):
                logger.info("App %s is too many, killing an instance." % app['name'])
                kid=sched.redis.rpop('docker_apps_%s_tasks' % app['name'])
                logger.info("Sending kill order for task % s" % kid)
                task = mesos_pb2.TaskID()
                task.value = str(kid)
                sched.driver.killTask(task)
                #TODO Don't assume success, try and try again
                sched.redis.hincrby('docker_apps_%s' % app['name'], 'running_instances', -1)
                #assuming LILO here, may not always be the case
                sched.redis.rpop('frontend:%s' % app['frontend'])

if __name__ == "__main__":
    logger.info("Connecting to %s" % sys.argv[1])

    frameworkDir = os.path.abspath(os.path.dirname(sys.argv[0]))
    execPath = os.path.join(frameworkDir, "docker_executor")

    execInfo = mesos_pb2.ExecutorInfo()
    execInfo.executor_id.value = "default"
    execInfo.command.value =  os.path.abspath("docker_executor.py")
    execInfo.name = "Docker Executor"
    execInfo.source = "Docker Project"

    r = redis.ConnectionPool(host=socket.gethostname(), port=6379, db=0)
    sched = DockerScheduler(execInfo, r)
    threading.Thread(target = monitor, args=[sched]).start()
    framework = mesos_pb2.FrameworkInfo()
    framework.user = "mesos" 
    framework.name = "Docker Framework"

    driver=mesos.MesosSchedulerDriver(sched, framework, sys.argv[1])
    sys.exit(0 if driver.run() == mesos_pb2.DRIVER_STOPPED else 1)
