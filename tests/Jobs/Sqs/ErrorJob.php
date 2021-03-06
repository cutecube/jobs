<?php

/**
 * Spiral Framework.
 *
 * @license   MIT
 * @author    Anton Titov (Wolfy-J)
 */

declare(strict_types=1);

namespace Spiral\Jobs\Tests\Sqs;

use Spiral\Jobs\JobHandler;

class ErrorJob extends JobHandler
{
    public function invoke(string $id): void
    {
        throw new \Error('something is wrong');
    }
}
