#!/usr/bin/env sh
groupadd pg_cat
groupadd pg_stabilize
groupadd pg_stabilized

usermod -a -G pg_cat,pg_stabilize,pg_stabilized `whoami`

useradd -M -N -g pg_cat -G pg_stabilized pu_cat
useradd -M -N -g pg_stabilize -G pg_stabilized pu_stabilize

mkdir not_shaky
chmod 510 not_shaky
chown pu_cat:pg_cat not_shaky

mkdir shaky
chmod 510 shaky
chown pu_stabilize:pg_stabilize shaky

mkdir stabilized
chmod 510 stabilized
chown `whoami`:pg_stabilized stabilized

