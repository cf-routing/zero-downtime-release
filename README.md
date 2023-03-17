# ARCHIVED -- DO NOT USE

## zero-downtime-release
Bosh release to test app availability over HTTP and TCP.

This was historically used to test TCP availability during upgrades to routing-release. This workflow was superseded by the use of 
[cloudfoundry/uptimer](https://github.com/cloudfoundry/uptimer) and the [cloudfoundry/cf-deployment-concourse-tasks](https://github.com/cloudfoundry/cf-deployment-concourse-tasks).

See the work done:
- https://www.pivotaltracker.com/story/show/183976643
- https://github.com/cloudfoundry/uptimer/pull/27
- https://github.com/cloudfoundry/cf-deployment-concourse-tasks/pull/151
