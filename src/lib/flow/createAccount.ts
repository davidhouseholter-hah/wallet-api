import * as fcl from "@onflow/fcl"
import * as t from "@onflow/types"
import * as Crypto from "../crypto"

import {
  ECDSA_P256,
  ECDSA_secp256k1,
  SHA2_256,
  SHA3_256,
  encodeKey,
} from "@onflow/util-encode-key"

import sendTransaction from "./sendTransaction"
import {AccountAuthorizer} from "./index"

const sigAlgos = {
  [Crypto.SignatureAlgorithm.ECDSA_P256]: ECDSA_P256,
  [Crypto.SignatureAlgorithm.ECDSA_secp256k1]: ECDSA_secp256k1,
}

const hashAlgos = {
  [Crypto.HashAlgorithm.SHA2_256]: SHA2_256,
  [Crypto.HashAlgorithm.SHA3_256]: SHA3_256,
}

const accountKeyWeight = 1000

const txCreateAccount = `
transaction(publicKey: String) {

  prepare(signer: AuthAccount) {
    let account = AuthAccount(payer: signer)
    account.addPublicKey(publicKey.decodeHex())
  }
}
`

export async function createAccount(
  publicKey: Crypto.PublicKey,
  sigAlgo: Crypto.SignatureAlgorithm,
  hashAlgo: Crypto.HashAlgorithm,
  authorization: AccountAuthorizer
): Promise<string> {
  const encodedPublicKey = encodeKey(
    publicKey.toHex(),
    sigAlgos[sigAlgo],
    hashAlgos[hashAlgo],
    accountKeyWeight
  )

  const result = await sendTransaction({
    transaction: txCreateAccount,
    args: [fcl.arg(encodedPublicKey, t.String)],
    authorizations: [authorization],
    payer: authorization,
    proposer: authorization,
  })

  const accountCreatedEvent = result.events[0].data

  return accountCreatedEvent.address
}