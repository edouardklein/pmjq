#!/usr/bin/env sh
groupadd pg_ffmpeg
groupadd pg_output

usermod -a -G pg_ffmpeg,pg_output `whoami`

useradd -M -N -g pg_ffmpeg -G pg_output pu_ffmpeg

mkdir audio
chmod 510 audio
chown pu_ffmpeg:pg_ffmpeg audio

mkdir output
chmod 510 output
chown `whoami`:pg_output output

mkdir video
chmod 510 video
chown pu_ffmpeg:pg_ffmpeg video

