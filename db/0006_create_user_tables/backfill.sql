create temporary table temp_users as (
    with users as (select distinct user_name from chatgpt_messages)
    select
        case
            when user_name = '' then 'teobot'
            else user_name
            end as mastodon_account_id,
        gen_random_uuid() as id
    from users
);

--------- Check contents before proceed -------

begin;

insert into users(id, name)
select id, mastodon_account_id from temp_users;

insert into mastodon_user_mappings(mastodon_account_id, user_id)
select mastodon_account_id, id from temp_users;

update chatgpt_messages set user_id = mum.user_id
from mastodon_user_mappings as mum
where mum.mastodon_account_id = case
    when chatgpt_messages.user_name = '' then 'teobot'
    else chatgpt_messages.user_name
end;

commit;