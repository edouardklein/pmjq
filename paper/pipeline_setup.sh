#!/usr/bin/env sh
groupadd pg_decode
groupadd pg_encode
groupadd pg_output
groupadd pg_stabilize
groupadd pg_watermark

usermod -a -G pg_decode,pg_encode,pg_output,pg_stabilize,pg_watermark `whoami`

useradd -M -N -g pg_decode -G pg_stabilize pu_decode
useradd -M -N -g pg_encode -G pg_output pu_encode
useradd -M -N -g pg_stabilize -G pg_watermark pu_stabilize
useradd -M -N -g pg_watermark -G pg_encode pu_watermark

mkdir decoded
chmod 510 decoded
chown pu_stabilize:pg_stabilize decoded

mkdir input
chmod 510 input
chown pu_decode:pg_decode input

mkdir output
chmod 510 output
chown `whoami`:pg_output output

mkdir stabilized
chmod 510 stabilized
chown pu_watermark:pg_watermark stabilized

mkdir watermarked
chmod 510 watermarked
chown pu_encode:pg_encode watermarked

