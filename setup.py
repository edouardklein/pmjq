from setuptools import setup

setup(name='pmjqtools',
      version='0.0.0',
      install_requires=[],
      description='Supporting tools for the PMJQ distributed computing\
      framework',
      url='',
      author='Edouard Klein',
      author_email='edouard.klein  -at- gmail.com',
      license='AGPL',
      packages=['pmjqtools'],
      entry_points={
          'console_scripts': ['pmjq_interactive=pmjqtools:pmjq_interactive',
                              'pmjq_viz=pmjqtools:pmjq_viz'],
      },)
