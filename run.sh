#!/bin/bash

export JWT_SECRET=secret
export STORAGE_MODE=s3
export S3_BUCKET=goenc
export S3_REGION=us-east-1
export S3_KEY_ID=minioadmin
export S3_KEY_SECRET=minioadmin
export S3_ENDPOINT=http://localhost:9000
export ENCODING_RESOLUTIONS="144p,240p,360p,480p,720p,1080p"
export API_KEY=verysecret
# export FFMPEG_HARDWARE_ACCEL=cuda
/home/danny/go/bin/air