Gluon - BOSH, CF, & Kubernetes
==============================

Ever wished you could manage your BOSH deployments and
Cloud Foundries from a Kubernetes control plane cluster?
**Gluon** might be just the thing for you and your
platform operations.

**Watch the Introductory Video**

[![Gluon Introductory Video](video.png)](http://www.youtube.com/watch?v=SVjxC3wMjMg "Gluon Introductory Video")


Installing Gluon on YOUR Kubernetes Cluster
-------------------------------------------

All fired up?  Ready to give Gluon a whirl on some Kubernets +
IaaS that you control?  Awesome.  To get started, we're going to
assume that you already have a Kubernetes cluster spun up
somewhere, that you have the ability to `kubectl ...` against it,
and that you have all the information you need to deploy a new
BOSH director.  If that last bit is a bit iffy, you can start
[with the BOSH.io docs][1] for [AWS][aws], [GCP][gcp],
[Azure][azure], or [ESX][esx].

[1]: https://bosh.cloudfoundry.org/docs/cli-v2-install/
[aws]: https://bosh.cloudfoundry.org/docs/init-aws/
[gcp]: https://bosh.cloudfoundry.org/docs/init-google/
[azure]: https://bosh.cloudfoundry.org/docs/init-azure/
[esx]: https://bosh.cloudfoundry.org/docs/init-vsphere/

The first thing you'll need to do is install cert-manager.  You
can do this via Helm, or by applying raw Kubernetes resource
specification YAMLs.  We personally prefer the latter:

    $ kubectl apply --validate=false -f https://github.com/jetstack/cert-manager/releases/download/v0.15.1/cert-manager.yaml

(from [the cert-manager installation docs][cert-manager])

[cert-manager]: https://cert-manager.io/docs/installation/kubernetes/

Once cert-manager is up, you'll need to provide it with a
certificate _issuer_.  For simplicity's sake, and for development
purposes and curiosity, we can just used a _self-signed issuer_:

    $ cat selfsigned-issuer.yml
    apiVersion: cert-manager.io/v1alpha2
    kind: ClusterIssuer
    metadata:
      name: internal-ca
      namespace: cert-manager
    spec:
      selfSigned: {}

    $ kubectl apply -f selfsigned-issuer.yml

This `internal-ca` cluster-wide certificate issuer will be used by
the validating and defaulting webhooks that you get (for free!)
with Gluon.  For real-world production use, you probably want
something like a [real, honest-to-goodness CA][cm-ca], a
[Vault-backed issuer][cm-vault], or an [ACME issuer (à la Let's
Encrypt)][cm-acme].

[cm-ca]:    https://cert-manager.io/docs/configuration/ca/
[cm-vault]: https://cert-manager.io/docs/configuration/vault/
[cm-acme]:  https://cert-manager.io/docs/configuration/acme/

Finally, it's time to install the Gluon custom resource
definitions (or "CRDs" to those hip Kubernetes cats), by applying
yet another YAML from the Internet &mdash; this time from this
very repository:

    $ kubectl apply -f https://raw.githubusercontent.com/starkandwayne/gluon/master/deploy/k8s.yml

With that configuration all done, you should be able to see the
new Gluon CRDs in the output of `kubectl api-resources`:

    $ kubectl api-resources | grep gluon
    boshconfigs      bcc            gluon.starkandwayne.com   true   BOSHConfig
    boshdeployments  bosh           gluon.starkandwayne.com   true   BOSHDeployment
    boshstemcells    stemcell,bsc   gluon.starkandwayne.com   true   BOSHStemcell

Now, you're all set!
