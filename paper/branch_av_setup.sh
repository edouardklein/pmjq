#!/usr/bin/env sh
groupadd pg_audio
groupadd pg_ffmpeg
groupadd pg_video

usermod -a -G pg_audio,pg_ffmpeg,pg_video `whoami`

useradd -M -N -g pg_ffmpeg -G pg_audio,pg_video pu_ffmpeg

mkdir audio
chmod 510 audio
chown `whoami`:pg_audio audio

mkdir decoded
chmod 510 decoded
chown pu_ffmpeg:pg_ffmpeg decoded

mkdir video
chmod 510 video
chown `whoami`:pg_video video

