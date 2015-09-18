#!/bin/bash
backup_dir=~/tmp/

cp $(which docker) $backup_dir/docker_backup$(date +"%m_%d_%y_%H_%M")
