# ccbank

![](./assets/sign.webp)

A Stacker News bot that lets you exchange credits for sats anonymously.

## How To Use

1. Create an **amountless** lightning invoice which is **valid for at least 24 hours**
2. Switch to [@anon](https://stacker.news/anon)
3. Mention the bot and include your lightning invoice:

> [@ccbank](https://stacker.news/ccbank) lnbc...

4. Wait for the bot to reply with a quote, for example:

> [@anon](https://stacker.news/anon) The current exchange rate is 2 credits per sat (max 10,000 sats).

This means the bot agrees to send you up to 10,000 sats for credits at the given exchange rate.

_If your lightning invoice includes an amount or is invalid for other reasons, the bot will reply
with an error._

5. Zap the bot from the account whose credits you want to sell.

> [!IMPORTANT]
> Due to how Stacker News works, the bot will only receive 70% of what you send. So for the bot to
> receive 100 credits, you need to send 100/0.7 ≈ 143 credits.

6. Wait for the payment

For security reasons, lightning invoices are not paid immediately for now. The bot notifies
[@ek](https://stacker.news/ek) to manually pay any lightning invoice. This means it can take up to
24 hours to get paid. If the payment doesn't arrive within 24 hours, please reach out to
[@ek](https://stacker.news/ek) on Stacker News or
[Signal](https://signal.me/#eu/QQJWrLHuZ-qRrNxo8x1CygWeU9ITJkrCkHg7Sm0vx4WfxB9y5PJM-aPINkauSLxb).

Once [@ek](https://stacker.news/ek) paid, the bot replies with a proof of payment. Anyone can use
this proof to verify that the bot pays out.

Future versions might use PGP for encryption to hide the recipient's node pubkey in the lightning
invoice. Only the payment hash needs to be public for payment verification.

