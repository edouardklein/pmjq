from setuptools import setup, find_packages

setup(name='pmjqtools',
      version='1.0.0β',
      install_requires=["daemux", "chan", "inotify", "python-dateutil",
                        'sphinxcontrib-programoutput', 'docopt'],
      description='Supporting tools for the PMJQ distributed computing\
      framework',
      url='http://edouardklein.github.io/pmjq/',
      author='Edouard Klein, Tristan Poupard',
      author_email='edouard.klein  -at- gmail.com, tristan.poupard -at- hotmail.com',
      license='AGPL',
      packages=['pmjqtools'],
      package_dir={'pmjqtools':
                   'pmjqtools'},
      include_package_data=True,
      package_data={},
      entry_points={
          'console_scripts': [
              'pmjq_viz=pmjqtools.dsl:viz',
              'pmjq_cmd=pmjqtools.dsl:cmd',
              'pmjq_mkendpoints=pmjqtools.tools:mkendpoints',
              'pmjq_start=pmjqtools.tools:start',
              'pmjq_sff=pmjqtools.pmjq_sff:launch',
          ],
      },)
