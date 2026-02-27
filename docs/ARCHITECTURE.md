# 🏗️ Arquitetura do Crom-Vision SaaS

O Crom-Vision evoluiu de um logger interno para uma plataforma de gerenciamento de Ativos Temporários (SaaS), viabilizando venda de rastreadores duradouros e seguros.

## Fluxo de Ciclo de Vida do Ativo
1. **Checkout**: O usuário envia uma requisição definindo o tempo desejado.
   - Planos Free (Ex: 1 dia) entram como Ativos (`payment_status = approved`, `is_private = false`).
   - Planos Pagos ficam Pendentes (`pending`, `is_private = true`).
2. **Ativação (Webhook)**: Serviço do MercadoPago notifica o pagamento concluído. O registro é ativado e uma senha de gestão é provisionada e "enviada" por e-mail.
3. **Consumo e Proxy**: Links operam através do proxy `/i/{id}` contabilizando hasheamentos restritos.
4. **Hard Delete Sweeper**: A cada hora, uma Task em Background localiza ativos onde o `expires_at` jaz no passado. Registros e Logs pertencentes a esse Link são irrevogavelmente destruídos do Banco SQLite.

## Segurança e Acesso
- Vitrine Pública: Somente recursos catalogados como plano Gratuito são acessados listados na raiz pública `index.html`.
- Painel Privado (`private.html`): Requer Inclusão de ID da imagem + Password gerada (E criptografada via SHA/Bcrypt na base) para habilitar as visualizações e relatórios geoespaciais.
