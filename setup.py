from setuptools import setup

setup(name='pmjqtools',
      version='0.0.1',
      install_requires=["daemux"],
      description='Supporting tools for the PMJQ distributed computing\
      framework',
      url='',
      author='Edouard Klein',
      author_email='edouard.klein  -at- gmail.com',
      license='AGPL',
      packages=['pmjqtools'],
      package_dir={'pmjqtools':
                   'pmjqtools'},
      include_package_data=True,
      package_data={},
      entry_points={
          'console_scripts': [
              # FIXME: Remove old entrypoints, replace with new python functions
              'pmjq_interactive=pmjqtools:pmjq_interactive',
              'pmjq_viz=pmjqtools:pmjq_viz'],
      },)
