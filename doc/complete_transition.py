complete_transition = {'inputs':
                       ['${PLAYGROUND}/input0/(?P<id>...)_(?P<suffix>.*)\.txt',
                        '${PLAYGROUND}/input1/(?P<prefix>.*)_(?P<id>...)\.txt'
                        ],
                       'errors': ['${PLAYGROUND}/error0/{{.Input 0}}',
                                  '${PLAYGROUND}/error1/{{.Input 1}}'],
                       'stderr': '${PLAYGROUND}/log/{{.Invariant}}.log',
                       'log': '${PLAYGROUND}/log/Arbitrary.log',
                       'id': 'Arbitrary',
                       'side_effects': ['Side_effect1',
                                        'Side_effect2'],
                       'outputs': ['${PLAYGROUND}/outputA/{{.NamedMatches.id}}'
                                   '_{{.NamedMatches.prefix}}_'
                                   '{{.NamedMatches.suffix}}.txt',
                                   '${PLAYGROUND}/outputB/{{.Invariant}}.txt'],
                       'quit_empty': True,
                       'invariant': '$id',
                       'cmd': 'sha256sum'}
