# ccbank

![](./assets/sign.webp)

A Stacker News bot that lets you exchange credits for sats anonymously.

## How To Use

1. Create a lightning invoice **with an amount > 1,000 sats** which is **valid for at least 24
   hours**
2. Switch to [@anon](https://stacker.news/anon)
3. Mention the bot and include your lightning invoice:

> [@ccbank](https://stacker.news/ccbank) sell lnbc...

4. Wait for the bot to reply with a quote, for example:

> [@anon](https://stacker.news/anon) Zap me 14286 credits, then I will pay your 5000 sats lightning
> invoice.
>
> <sub>exchange rate: 2.86 credits/sat · you send 14286 credits · bot receives 10000 credits ·
> payments can take up to 24h</sub>

As the footnote mentions, the bot will include the 30% fee between users in the amount you should
send.

_If your lightning invoice is amountless or is invalid for other reasons, the bot will reply with an
error._

5. Zap the bot the quoted amount from the account whose credits you want to sell.

6. Wait for the payment

For security reasons, lightning invoices are not paid immediately for now. The bot notifies
[@ek](https://stacker.news/ek) to manually pay any lightning invoice. This means it can take up to
24 hours to get paid. If the payment doesn't arrive within 24 hours, please reach out to
[@ek](https://stacker.news/ek) on Stacker News or
[Signal](https://signal.me/#eu/QQJWrLHuZ-qRrNxo8x1CygWeU9ITJkrCkHg7Sm0vx4WfxB9y5PJM-aPINkauSLxb).

Once [@ek](https://stacker.news/ek) paid, the bot replies with a proof of payment. Anyone can use
this proof to verify that the bot pays out.

Future versions might use PGP for encryption to hide the recipient's node pubkey in the lightning
invoice. Only the payment hash and amount need to be public for payment verification.

