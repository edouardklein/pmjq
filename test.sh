#!/usr/bin/env sh
pmjq --inputs "(.*)" md5ed sha1ed --cmd "cat \$1 \$2 > output/\`basename \$1\`"
pmjq --inputs "(.*)" input --cmd "cp \$1 md5_pool/ && cp \$1 sha1_pool/"
pmjq md5_pool md5sum md5ed
pmjq sha1_pool sha1sum sha1ed
