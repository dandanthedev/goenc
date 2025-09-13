#!/bin/bash

export JWT_SECRET=secret
export STORAGE_MODE=s3
export S3_BUCKET=goenc
export S3_REGION=us-east-1
export S3_KEY_ID=minioadmin
export S3_KEY_SECRET=minioadmin
export S3_ENDPOINT=http://localhost:9000
export LOCAL_STORAGE_PATH=localdata/
export ENCODING_RESOLUTIONS="144p,240p,360p,480p,720p,1080p"
export API_KEY=verysecret
export REDIS_ADDR=localhost:6379
export REDIS_PASSWORD=
export REDIS_DB=0
export TASKS=encode,server,stuckrecovery # comma separated list of tasks this worker should do
export STUCKRECOVERY_CRON="0 0 * * *"
# export FFMPEG_HARDWARE_ACCEL=cuda
/home/danny/go/bin/air