from shlex import quote as q

complete_transition = {'inputs':
                       ['/tmp/input0/'+q('(?P<id>...)_(?P<suffix>.*)\.txt'),
                        '/tmp/input1/'+q('(?P<prefix>.*)_(?P<id>...)\.txt'),
                        ],
                       'errors': ['/tmp/error0/'+q('{{.Input 0}}'),
                                  '/tmp/error1/'+q('{{.Input 1}}')],
                       'stderr': '/tmp/log/{{.Invariant}}.log',
                       'log': '/tmp/log/Arbitrary.log',
                       'id': 'Arbitrary',
                       'side_effects': ['Side_effect1',
                                        'Side_effect2'],
                       'outputs': ['/tmp/outputA/{{.NamedMatches.id}}'
                                   '_{{.NamedMatches.prefix}}_'
                                   '{{.NamedMatches.suffix}}.txt',
                                   '/tmp/outputB/{{.Invariant}}.txt'],
                       'quit_empty': True,
                       'invariant': '$id',
                       'cmd': 'sha256sum'}
