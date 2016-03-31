#!/usr/bin/env python3
import pexpect
import subprocess
import time
import shutil

pmjq_interactive_cmd = 'coverage run --append '+shutil.which('pmjq_interactive')
pmjq_viz_cmd = 'coverage run --append '+shutil.which('pmjq_viz')

print('Testing a simple chained pipeline')
pmjq = pexpect.spawn(pmjq_interactive_cmd,
                     timeout=10, logfile=open('/dev/stdout', 'wb'))
pmjq.expect_exact('Command:')
pmjq.sendline('decode')
pmjq.expect_exact('Input dir(s):')
pmjq.sendline('input')
pmjq.expect_exact('Output dir(s):')
pmjq.sendline('decoded')

pmjq.expect_exact('Command:')
pmjq.sendline('stabilize')
pmjq.expect_exact('Input dir(s):')
pmjq.sendline('decoded')
pmjq.expect_exact('Output dir(s):')
pmjq.sendline('stabilized')

pmjq.expect_exact('Command:')
pmjq.sendline('watermark')
pmjq.expect_exact('Input dir(s):')
pmjq.sendline('stabilized')
pmjq.expect_exact('Output dir(s):')
pmjq.sendline('watermarked')

pmjq.expect_exact('Command:')
pmjq.sendline('encode --format mp4')
pmjq.expect_exact('Input dir(s):')
pmjq.sendline('watermarked')
pmjq.expect_exact('Output dir(s):')
pmjq.sendline('output')

pmjq.expect_exact('Command:')
pmjq.sendline('')

expected_setup_sh = '''#!/usr/bin/env sh
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

'''

time.sleep(1)
assert(open('setup.sh').read() == expected_setup_sh)

expected_launch_sh = '''#!/usr/bin/env sh
sudo -u pu_decode pmjq input decode decoded
sudo -u pu_encode pmjq watermarked "encode --format mp4" output
sudo -u pu_stabilize pmjq decoded stabilize stabilized
sudo -u pu_watermark pmjq stabilized watermark watermarked
'''

assert(open('launch.sh').read() == expected_launch_sh)

expected_cleanup_sh = '''#!/usr/bin/env sh
groupdel pg_decode
groupdel pg_encode
groupdel pg_output
groupdel pg_stabilize
groupdel pg_watermark

userdel pu_decode
userdel pu_encode
userdel pu_stabilize
userdel pu_watermark

rm -r decoded
rm -r input
rm -r output
rm -r stabilized
rm -r watermarked
'''

assert(open('cleanup.sh').read() == expected_cleanup_sh)

print('Testing the visualization tool...')
# Dot<->Petri net template from http://thegarywilson.com/blog/2011/drawing-petri-nets/
expected_viz = '''digraph G {
subgraph place {
graph [shape=circle,color=gray];
node [shape=circle];
"decoded";
"input";
"output";
"stabilized";
"watermarked";
}

subgraph transitions {
node [shape=rect];
"decode";
"encode";
"stabilize";
"watermark";
}

"decoded"->"stabilize";
"input"->"decode";
"stabilized"->"watermark";
"watermarked"->"encode";

"decode"->"decoded";
"encode"->"output";
"stabilize"->"stabilized";
"watermark"->"watermarked";
}
'''

time.sleep(1)
assert(subprocess.check_output(pmjq_viz_cmd + ' < setup.sh', shell=True).decode('utf8') ==
       expected_viz)


print('Testing branching')
pmjq = pexpect.spawn(pmjq_interactive_cmd, timeout=10, logfile=open('/dev/stdout', 'wb'))
pmjq.expect_exact('Command:')
pmjq.sendline('ffmpeg -i $1 -map 0:v -vcodec copy video/`basename $1`.ogv -map 0:a -acodec copy audio/`basename $1`.ogg')
pmjq.expect_exact('Input dir(s):')
pmjq.sendline('decoded')
pmjq.expect_exact('Output dir(s):')
pmjq.sendline('audio video')

pmjq.expect_exact('Command:')
pmjq.sendline('')

expected_setup_sh = '''#!/usr/bin/env sh
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

'''

time.sleep(1)

assert(open('setup.sh', 'r').read() == expected_setup_sh)


print('Testing and-merging')
pmjq = pexpect.spawn(pmjq_interactive_cmd, timeout=10, logfile=open('/dev/stdout', 'wb'))
pmjq.expect_exact('Command:')
pmjq.sendline('ffmpeg -i $1 -i $2 -c copy output/`basename $1`.ogg')
pmjq.expect_exact('Input dir(s):')
pmjq.sendline('audio video')
pmjq.expect_exact('Pattern (default "(.*)"):')
pmjq.sendline('(.*).og[gv]$')
pmjq.expect_exact('Output dir(s):')
pmjq.sendline('output')

pmjq.expect_exact('Command:')
pmjq.sendline('')

expected_setup_sh = '''#!/usr/bin/env sh
groupadd pg_ffmpeg_0
groupadd pg_ffmpeg_1
groupadd pg_output

usermod -a -G pg_ffmpeg_0,pg_ffmpeg_1,pg_output `whoami`

useradd -M -N -g pg_ffmpeg_0 -G pg_ffmpeg_1,pg_output pu_ffmpeg

mkdir audio
chmod 510 audio
chown pu_ffmpeg:pg_ffmpeg_0 audio

mkdir output
chmod 510 output
chown `whoami`:pg_output output

mkdir video
chmod 510 video
chown pu_ffmpeg:pg_ffmpeg_1 video

'''

time.sleep(1)

assert(open('setup.sh', 'r').read() == expected_setup_sh)


print('Testing the visualization tool again...')
# Dot<->Petri net template from http://thegarywilson.com/blog/2011/drawing-petri-nets/
expected_viz = '''digraph G {
subgraph place {
graph [shape=circle,color=gray];
node [shape=circle];
"audio";
"output";
"video";
}

subgraph transitions {
node [shape=rect];
"ffmpeg";
}

"audio"->"ffmpeg";
"video"->"ffmpeg";

"ffmpeg"->"output";
}
'''

time.sleep(1)
assert(subprocess.check_output(pmjq_viz_cmd + ' < setup.sh', shell=True).decode('utf8') ==
       expected_viz)

print('Testing xor-merging')
pmjq = pexpect.spawn(pmjq_interactive_cmd, timeout=10, logfile=open('/dev/stdout', 'wb'))
pmjq.expect_exact('Command:')
pmjq.sendline('cat')
pmjq.expect_exact('Input dir(s):')
pmjq.sendline('not_shaky')
pmjq.expect_exact('Output dir(s):')
pmjq.sendline('stabilized')

pmjq.expect_exact('Command:')
pmjq.sendline('stabilize')
pmjq.expect_exact('Input dir(s):')
pmjq.sendline('shaky')
pmjq.expect_exact('Output dir(s):')
pmjq.sendline('stabilized')

pmjq.expect_exact('Command:')
pmjq.sendline('')

expected_setup_sh = '''#!/usr/bin/env sh
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

'''

time.sleep(1)

assert(open('setup.sh', 'r').read() == expected_setup_sh)


print('Testing the whole example network')
subprocess.call("grep -v -E '^#' < ../paper/whole_pipeline.txt |" + pmjq_interactive_cmd,
                shell=True)

expected_setup_sh='''#!/usr/bin/env sh
groupadd pg_equalizer
groupadd pg_ffmpeg_7d8_0
groupadd pg_ffmpeg_7d8_1
groupadd pg_ffmpeg_d15
groupadd pg_is_shaky
groupadd pg_output
groupadd pg_stabilize

usermod -a -G pg_equalizer,pg_ffmpeg_7d8_0,pg_ffmpeg_7d8_1,pg_ffmpeg_d15,pg_is_shaky,pg_output,pg_stabilize `whoami`

useradd -M -N -g pg_equalizer -G pg_ffmpeg_7d8_0 pu_equalizer
useradd -M -N -g pg_ffmpeg_7d8_0 -G pg_ffmpeg_7d8_1,pg_output pu_ffmpeg_7d8
useradd -M -N -g pg_ffmpeg_d15 -G pg_equalizer,pg_is_shaky pu_ffmpeg_d15
useradd -M -N -g pg_is_shaky -G pg_stabilize,pg_ffmpeg_7d8_1 pu_is_shaky
useradd -M -N -g pg_stabilize -G pg_ffmpeg_7d8_1 pu_stabilize

mkdir audio
chmod 510 audio
chown pu_equalizer:pg_equalizer audio

mkdir equalized
chmod 510 equalized
chown pu_ffmpeg_7d8:pg_ffmpeg_7d8_0 equalized

mkdir input
chmod 510 input
chown pu_ffmpeg_d15:pg_ffmpeg_d15 input

mkdir output
chmod 510 output
chown `whoami`:pg_output output

mkdir shaky
chmod 510 shaky
chown pu_stabilize:pg_stabilize shaky

mkdir stabilized
chmod 510 stabilized
chown pu_ffmpeg_7d8:pg_ffmpeg_7d8_1 stabilized

mkdir video
chmod 510 video
chown pu_is_shaky:pg_is_shaky video

'''

time.sleep(1)
assert(open('setup.sh', 'r').read() == expected_setup_sh)
